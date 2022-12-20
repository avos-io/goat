package goat_test

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/avos-io/goat"
	wrapped "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"nhooyr.io/websocket"
)

type simulatedServer struct {
	srv *goat.Server
}

func newSimulatedServer(id string, transport goat.RpcReadWriter) *simulatedServer {
	srv := goat.NewServer(id)

	go func() {
		err := srv.Serve(transport)
		panic(err)
	}()

	return &simulatedServer{srv}
}

type oneToOneProxy struct {
	t1, t2 goat.RpcReadWriter
}

func newOneToOneProxy(t1, t2 goat.RpcReadWriter) *oneToOneProxy {
	proxy := &oneToOneProxy{t1, t2}

	go proxy.forward(t1, t2)
	go proxy.forward(t2, t1)

	return proxy
}

func (p *oneToOneProxy) forward(from, to goat.RpcReadWriter) {
	ctx := context.Background()

	for {
		rpc, err := from.Read(ctx)
		if err != nil {
			panic(err)
		}

		err = to.Write(ctx, rpc)
		if err != nil {
			panic(err)
		}
	}
}

type manyToOneProxy struct {
	mutex        sync.Mutex
	backend      goat.RpcReadWriter
	clientInput  chan *wrapped.Rpc
	clientOutput map[string]chan *wrapped.Rpc
}

func newManyToOneProxy(backend goat.RpcReadWriter) *manyToOneProxy {
	p := &manyToOneProxy{
		backend:      backend,
		clientInput:  make(chan *wrapped.Rpc),
		clientOutput: make(map[string]chan *wrapped.Rpc),
	}
	go p.serveWrites()
	go p.serveReads()
	return p
}

func (p *manyToOneProxy) serveWrites() {
	for {
		rpc := <-p.clientInput
		err := p.backend.Write(context.Background(), rpc)
		if err != nil {
			panic(err)
		}
	}
}

func (p *manyToOneProxy) serveReads() {
	for {
		rpc, err := p.backend.Read(context.Background())
		if err != nil {
			panic(err)
		}

		destAddr := rpc.Header.Destination

		if destAddr == "" {
			log.Println("No dest header, ignoring rpc")
			continue
		}

		p.mutex.Lock()
		clientChan, ok := p.clientOutput[destAddr]
		p.mutex.Unlock()

		if !ok {
			log.Println("no such client")
			continue
		}

		clientChan <- rpc
	}
}

func (p *manyToOneProxy) AddClient(id string, c goat.RpcReadWriter) {
	myChan := make(chan *wrapped.Rpc)

	p.mutex.Lock()
	p.clientOutput[id] = myChan
	p.mutex.Unlock()

	go func() {
		rpc, err := c.Read(context.Background())
		if err != nil {
			panic(err)
		}

		sourceAddr := rpc.Header.Source

		if sourceAddr != id {
			panic("invalid source addr from client")
		}

		p.clientInput <- rpc
	}()

	go func() {
		rpc := <-myChan

		err := c.Write(context.Background(), rpc)
		if err != nil {
			panic(err)
		}
	}()
}

type simulatedClient struct {
	transport goat.RpcReadWriter
}

func newSimulatedClient(transport goat.RpcReadWriter) *simulatedClient {
	return &simulatedClient{transport}
}

func (c *simulatedClient) newClientConn(source, dest string) grpc.ClientConnInterface {
	return goat.NewClientConn(c.transport, source, dest)
}

type echoServer struct {
	testproto.UnimplementedTestServiceServer
}

func (*echoServer) Unary(_ context.Context, m *testproto.Msg) (*testproto.Msg, error) {
	return m, nil
}

// An end to end test of the following topology:
//
// [Client] <-> [Proxy] <-> [Server]

func TestGoatOverPipesSingleClientSingleServer(t *testing.T) {
	// Make a pipe for communication between Proxy and Server
	ps1, ps2 := net.Pipe()
	// And between Client and Proxy
	ps3, ps4 := net.Pipe()

	simServer := newSimulatedServer("s:1", testutil.NewGoatOverPipe(ps1))
	echoServer := &echoServer{}
	testproto.RegisterTestServiceServer(simServer.srv, echoServer)

	_ = newOneToOneProxy(testutil.NewGoatOverPipe(ps2), testutil.NewGoatOverPipe(ps3))

	simClient := newSimulatedClient(testutil.NewGoatOverPipe(ps4))

	tpClient := testproto.NewTestServiceClient(simClient.newClientConn("src", "s:1"))
	result, err := tpClient.Unary(context.Background(), &testproto.Msg{Value: 11})
	if err != nil {
		panic(err)
	}

	require.Equal(t, int32(11), result.Value)
}

func TestGoatOverPipesManyClientsSingleServer(t *testing.T) {
	// Make a pipe for communication between Proxy and Server
	ps1, ps2 := net.Pipe()

	simServer := newSimulatedServer("server0", testutil.NewGoatOverPipe(ps1))
	echoServer := &echoServer{}
	testproto.RegisterTestServiceServer(simServer.srv, echoServer)

	proxy := newManyToOneProxy(testutil.NewGoatOverPipe(ps2))

	numClients := 100

	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		clientAddress := fmt.Sprintf("client:%08x", i)

		go func(v int) {
			ps3, ps4 := net.Pipe()
			proxy.AddClient(clientAddress, testutil.NewGoatOverPipe(ps3))
			simClient := newSimulatedClient(testutil.NewGoatOverPipe(ps4))

			tpClient := testproto.NewTestServiceClient(
				simClient.newClientConn(clientAddress, "server0"),
			)
			result, err := tpClient.Unary(context.Background(), &testproto.Msg{Value: 11})
			if err != nil {
				panic(err)
			}

			require.Equal(t, int32(11), result.Value)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

// Websocket between Client and Proxy, pipe betwene Proxy and Server
func TestGoatOverWebsocketsSingleClientSingleServer(t *testing.T) {
	ps1, ps2 := net.Pipe()

	simServer := newSimulatedServer("s:1", testutil.NewGoatOverPipe(ps1))
	echoServer := &echoServer{}
	testproto.RegisterTestServiceServer(simServer.srv, echoServer)

	proxy := newManyToOneProxy(testutil.NewGoatOverPipe(ps2))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			panic(err)
		}

		clientAddress := "badf00d"
		proxy.AddClient(clientAddress, goat.NewGoatOverWebsocket(c))

		// Something should take ownership of closing the connection; we don't do that
		// yet in these tests.
		//c.Close(websocket.StatusNormalClosure, "")
	})

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	go func() {
		err = http.Serve(listener, handler)
		if err != nil {
			panic(err)
		}
	}()

	conn, _, err := websocket.Dial(
		context.Background(),
		fmt.Sprintf("ws://%s", listener.Addr().String()),
		nil,
	)
	if err != nil {
		panic(err)
	}

	simClient := newSimulatedClient(goat.NewGoatOverWebsocket(conn))

	tpClient := testproto.NewTestServiceClient(simClient.newClientConn("badf00d", "s:1"))
	result, err := tpClient.Unary(context.Background(), &testproto.Msg{Value: 11})
	if err != nil {
		panic(err)
	}

	require.Equal(t, int32(11), result.Value)
}

func TestRealProxy(t *testing.T) {
	const (
		proxyAddress  = "cloud:1"
		serverAddress = "cloud:2"
		serviceName   = "service:sam-was-here"
		clientAddress = "client:1"
	)

	srv := goat.NewServer(serverAddress)
	echoServer := &echoServer{}
	testproto.RegisterTestServiceServer(srv, echoServer)

	proxy := goat.NewProxy(
		proxyAddress,
		func(id string) (goat.RpcReadWriter, error) {
			if id == serverAddress {
				ps3, ps4 := net.Pipe()

				go srv.Serve(testutil.NewGoatOverPipe(ps4))

				return testutil.NewGoatOverPipe(ps3), nil
			}

			return nil, fmt.Errorf("invalid ID to connect to")
		},
		func(hdr *wrapped.RequestHeader) error {
			if hdr.Destination == serviceName {
				// It would be reasonable to look up client metadata like org at this
				// point. Assuming we save such on client connection, then it could
				// simply be a map lookup based on hdr.Source. Alternatively, it's
				// a case of looking through hdr.Headers.
				hdr.Destination = serverAddress
			}
			return nil
		},
		nil)

	go proxy.Serve()

	ps1, ps2 := net.Pipe()

	proxy.AddClient(clientAddress, testutil.NewGoatOverPipe(ps2))

	simClient := newSimulatedClient(testutil.NewGoatOverPipe(ps1))
	tpClient := testproto.NewTestServiceClient(simClient.newClientConn(clientAddress, serviceName))

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*2))
	defer cancel()

	result, err := tpClient.Unary(ctx, &testproto.Msg{Value: 11})
	if err != nil {
		panic(err)
	}

	require.Equal(t, int32(11), result.Value)
}
