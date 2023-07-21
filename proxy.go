package goat

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"

	wrapped "github.com/avos-io/goat/gen"
)

const (
	clientBufferSize = 16
)

// Is free to modify the passed in header, e.g. changing the Destination.
// If an error is returned, the RPC is dropped.
type RpcIntercepter func(hdr *wrapped.RequestHeader) error
type NewConnection func(id string) (RpcReadWriter, error)
type ClientDisconnect func(id string, reason error)

type Proxy struct {
	id               string
	newConnection    NewConnection
	clientDisconnect ClientDisconnect
	rpcIntercepter   RpcIntercepter

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
	conn       RpcReadWriter
	toServer   chan command
	fromServer chan *wrapped.Rpc
}

func NewProxy(
	id string,
	newConnection NewConnection,
	rpcIntercepter RpcIntercepter,
	onClientDisconnect ClientDisconnect,
) *Proxy {
	return &Proxy{
		id:               id,
		newConnection:    newConnection,
		rpcIntercepter:   rpcIntercepter,
		clientDisconnect: onClientDisconnect,
		commands:         make(chan command),
		clients:          make(map[string]*proxyClient),
	}
}

func (p *Proxy) AddClient(id string, conn RpcReadWriter) {
	log.Info().Msgf("proxy.AddClient %s", id)

	client := &proxyClient{
		id:         id,
		conn:       conn,
		toServer:   p.commands,
		fromServer: make(chan *wrapped.Rpc, clientBufferSize),
	}

	p.mutex.Lock()
	p.clients[id] = client
	p.mutex.Unlock()

	go client.readLoop()
	go client.writeLoop()
}

func (p *Proxy) addOutgoingConnectionLocked(id string) *proxyClient {
	log.Info().Msgf("proxy.addOutgoingConnectionLocked %s", id)

	client := &proxyClient{
		id:         id,
		toServer:   p.commands,
		fromServer: make(chan *wrapped.Rpc, clientBufferSize),
	}
	p.clients[id] = client

	go client.connect(p.newConnection)

	return client
}

func (p *Proxy) Serve(ctx context.Context) {
	// For performance reasons, is it sane to have many instances of serveClients() running at once?
	// Maybe we could fire up e.g. 8 of them.

	p.serveClients(ctx)
}

func (p *Proxy) serveClients(ctx context.Context) {
	for {
		select {
		case cmd := <-p.commands:
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
		case <-ctx.Done():
			log.Warn().Msg("serveClients context cancelled")
			return
		}
	}
}

func (p *Proxy) forwardRpc(source string, rpc *wrapped.Rpc) {
	// Sanity check RPC first
	if rpc.Header == nil || rpc.Header.Source != source {
		log.Warn().Msgf("Bad Rpc: %v", rpc)
		log.Panic().Msg("TODO: handle invalid RPC here (log and ignore?)")
	}

	// Apply any sort of address translation first: this allows implementing a
	// NAT or DNS like functionality on top of this library.
	if p.rpcIntercepter != nil {
		err := p.rpcIntercepter(rpc.Header)
		if err != nil {
			return
		}
	}

	destination := rpc.Header.Destination

	// XXX: this proxy stuff might be of no value -- consider removing it or at least
	// making it optional.
	if true {
		// Mark the proxy route this RPC is taking so responses can be routed back
		// via the same path.
		rpc.Header.ProxyRecord = append(rpc.Header.ProxyRecord, p.id)

		// If there is a proxy route we're following, use that as the destination
		// address in preference to the one marked in the header.
		if rpc.Header.ProxyNext != nil {
			destination = rpc.Header.ProxyNext[len(rpc.Header.ProxyNext)-1]
			rpc.Header.ProxyNext = rpc.Header.ProxyNext[0 : len(rpc.Header.ProxyNext)-1]
		}
	}

	// Now try and forward to the destination we've decided on.
	p.mutex.Lock()
	client, ok := p.clients[destination]
	if !ok {
		client = p.addOutgoingConnectionLocked(destination)
	}
	p.mutex.Unlock()

	select {
	case client.fromServer <- rpc:
	default:
		log.Warn().Str("source", rpc.Header.Source).
			Str("destination", rpc.Header.Destination).
			Str("method", rpc.Header.Method).
			Msgf("Dropping packet")
	}
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
