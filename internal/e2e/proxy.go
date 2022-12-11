package e2e

import (
	"context"
	"log"
	"sync"

	"github.com/avos-io/goat"
	wrapped "github.com/avos-io/goat/gen"
)

type ClientDisconnect func(id string, reason error)

// TODO: in practice we want version info or similar to to able to construct a
// version-specific service URL -- need to think about how that information is
// flowed about the system.
type DestinationAddressResolver func(destination string) string

type NewConnection func(id string) (goat.RpcReadWriter, error)

type proxy struct {
	id               string
	newConnection    NewConnection
	clientDisconnect ClientDisconnect
	destAddrResolver DestinationAddressResolver

	mutex    sync.Mutex
	clients  map[string]*proxyClient
	commands chan command
}

type command struct {
	id  string
	rpc *wrapped.Rpc
	err error
}

type proxyClient struct {
	id         string
	conn       goat.RpcReadWriter
	toServer   chan command
	fromServer chan *wrapped.Rpc
}

func NewProxy(
	id string,
	newConnection NewConnection,
	resolveDestinationAddress DestinationAddressResolver,
	onClientDisconnect ClientDisconnect,
) *proxy {
	return &proxy{
		id:               id,
		newConnection:    newConnection,
		destAddrResolver: resolveDestinationAddress,
		clientDisconnect: onClientDisconnect,
		commands:         make(chan command),
		clients:          make(map[string]*proxyClient),
	}
}

func (p *proxy) AddClient(id string, conn goat.RpcReadWriter) {
	client := &proxyClient{
		id:         id,
		conn:       conn,
		toServer:   p.commands,
		fromServer: make(chan *wrapped.Rpc),
	}

	p.mutex.Lock()
	p.clients[id] = client
	p.mutex.Unlock()

	go client.readLoop()
	go client.writeLoop()
}

func (p *proxy) addOutgoingConnectionLocked(id string) *proxyClient {
	client := &proxyClient{
		id:         id,
		toServer:   p.commands,
		fromServer: make(chan *wrapped.Rpc),
	}
	p.clients[id] = client

	go client.connect(p.newConnection)

	return client
}

func (p *proxy) Serve() {
	// For performance reasons, is it sane to have many instances of serveClients() running at once?
	// Maybe we could fire up e.g. 8 of them.

	p.serveClients()
}

func (p *proxy) serveClients() {
	for {
		cmd := <-p.commands

		if cmd.rpc != nil {
			p.forwardRpc(cmd.id, cmd.rpc)
		} else if cmd.err != nil {
			p.mutex.Lock()
			delete(p.clients, cmd.id)
			p.mutex.Unlock()
			if p.clientDisconnect != nil {
				p.clientDisconnect(cmd.id, cmd.err)
			}
		}
	}
}

func (p *proxy) forwardRpc(source string, rpc *wrapped.Rpc) {
	// Sanity check RPC first
	if rpc.Header == nil || rpc.Header.Source != source {
		log.Panicf("TODO: handle invalid RPC here (log and ignore?)")
	}

	// Apply any sort of address translation first: this allows implementing a
	// NAT or DNS like functionality on top of this library.
	if p.destAddrResolver != nil {
		rpc.Header.Destination = p.destAddrResolver(rpc.Header.Destination)
	}

	// Mark the proxy route this RPC is taking so responses can be routed back
	// via the same path.
	rpc.Header.ProxyRecord = append(rpc.Header.ProxyRecord, p.id)

	destination := rpc.Header.Destination

	// If there is a proxy route we're following, use that as the destination
	// address in preference to the one marked in the header.
	if rpc.Header.ProxyNext != nil {
		destination = rpc.Header.ProxyNext[len(rpc.Header.ProxyNext)-1]
		rpc.Header.ProxyNext = rpc.Header.ProxyNext[0 : len(rpc.Header.ProxyNext)-1]
	}

	// TODO: allow a callback function to implement routing policy: decide if
	// we're allowed to send a packet between these two ids.

	// Now try and forward to the destination we've decided on.
	p.mutex.Lock()
	client, ok := p.clients[destination]
	if !ok {
		client = p.addOutgoingConnectionLocked(destination)
	}
	p.mutex.Unlock()

	client.fromServer <- rpc
}

func (c *proxyClient) readLoop() {
	ctx := context.Background()
	for {
		rpc, err := c.conn.Read(ctx)
		if err != nil {
			c.toServer <- command{id: c.id, err: err}
			return
		}

		c.toServer <- command{id: c.id, rpc: rpc}
	}
}

func (c *proxyClient) writeLoop() {
	ctx := context.Background()
	for {
		rpc := <-c.fromServer

		err := c.conn.Write(ctx, rpc)
		if err != nil {
			c.toServer <- command{id: c.id, err: err}
			return
		}
	}
}

func (c *proxyClient) connect(newConnection NewConnection) {
	var err error

	c.conn, err = newConnection(c.id)
	if err != nil {
		c.toServer <- command{id: c.id, err: err}
		return
	}

	go c.readLoop()
	go c.writeLoop()
}
