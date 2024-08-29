package goat_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/avos-io/goat"
	"github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type simulatedServer struct {
	srv *goat.Server
}

func newSimulatedServer(id string, transport goat.RpcReadWriter) *simulatedServer {
	srv := goat.NewServer(id)

	go func() {
		err := srv.Serve(context.Background(), transport)
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
	clientInput  chan *goatorepo.Rpc
	clientOutput map[string]chan *goatorepo.Rpc
}

func newManyToOneProxy(backend goat.RpcReadWriter) *manyToOneProxy {
	p := &manyToOneProxy{
		backend:      backend,
		clientInput:  make(chan *goatorepo.Rpc),
		clientOutput: make(map[string]chan *goatorepo.Rpc),
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
	myChan := make(chan *goatorepo.Rpc)

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

	pctx, pcancel := context.WithCancel(context.Background())
	defer pcancel()

	proxy := goat.NewProxy(
		pctx,
		proxyAddress,
		func(id string) (goat.RpcReadWriter, error) {
			if id == serverAddress {
				ps3, ps4 := net.Pipe()

				go srv.Serve(context.Background(), testutil.NewGoatOverPipe(ps4))

				return testutil.NewGoatOverPipe(ps3), nil
			}

			return nil, fmt.Errorf("invalid ID to connect to")
		},
		func(hdr *goatorepo.RequestHeader) error {
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

func TestContextCancelled(t *testing.T) {
	const (
		clientAddress1 = "client:1"
	)

	client1Read := make(chan *goat.Rpc)
	client1Write := make(chan *goat.Rpc)
	client1Rw := goat.NewGoatOverChannel(client1Write, client1Read)

	ctx, cancel := context.WithCancel(context.Background())

	p := goat.NewProxy(
		ctx,
		"me",
		func(id string) (goat.RpcReadWriter, error) {
			return nil, errors.New("You fool")
		},
		func(hdr *goatorepo.RequestHeader) error { return nil },
		func(id string, reason error) {},
	)

	p.AddClient(clientAddress1, client1Rw)

	go p.Serve()

	client1Write <- &goatorepo.Rpc{
		Id: 1,
		Header: &goatorepo.RequestHeader{
			Source:      clientAddress1,
			Destination: clientAddress1,
		},
	}

	// We should receive this rpc
	select {
	case <-client1Read:
	case <-time.After(1 * time.Second):
		t.FailNow()
	}

	cancel()

	select {
	case client1Write <- &goatorepo.Rpc{
		Id: 2,
		Header: &goatorepo.RequestHeader{
			Source:      clientAddress1,
			Destination: clientAddress1,
		},
	}:
		t.FailNow() // Should no longer be reading from this channel
	default:
	}
}

func TestReadErrorClosesBothLoops(t *testing.T) {
	const (
		clientAddress1 = "client:1"
		clientAddress2 = "client:2"
	)

	doneChannel := make(chan struct{})

	client1Rw := mocks.NewMockRpcReadWriter(t)
	client1Rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errors.New("error"))
	client1Rw.EXPECT().Read(mock.Anything).Run(func(ctx context.Context) {
		<-ctx.Done()
		doneChannel <- struct{}{}
	}).Return(nil, errors.New("Context cancelled"))

	client2Read := make(chan *goat.Rpc)
	client2Write := make(chan *goat.Rpc)
	client2Rw := goat.NewGoatOverChannel(client2Write, client2Read)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := goat.NewProxy(
		ctx,
		"me",
		func(id string) (goat.RpcReadWriter, error) {
			return nil, errors.New("You fool")
		},
		func(hdr *goatorepo.RequestHeader) error { return nil },
		func(id string, reason error) {},
	)

	p.AddClient(clientAddress1, client1Rw)
	p.AddClient(clientAddress2, client2Rw)

	go p.Serve()

	client2Write <- &goatorepo.Rpc{
		Id: 1,
		Header: &goatorepo.RequestHeader{
			Source:      clientAddress2,
			Destination: clientAddress1,
		},
	}

	select {
	case <-doneChannel:
	case <-time.After(1 * time.Second):
		t.FailNow()
	}
}

func TestUnresponsiveClient(t *testing.T) {
	const (
		clientAddress1 = "client:1"
		clientAddress2 = "client:2"
	)

	client1Read := make(chan *goat.Rpc)
	client1Write := make(chan *goat.Rpc)
	client1Rw := goat.NewGoatOverChannel(client1Write, client1Read)

	client2Read := make(chan *goat.Rpc)
	client2Write := make(chan *goat.Rpc)
	client2Rw := goat.NewGoatOverChannel(client2Write, client2Read)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := goat.NewProxy(
		ctx,
		"me",
		func(id string) (goat.RpcReadWriter, error) {
			println("Asking to connect to ", id)
			return nil, errors.New("You fool")
		},
		func(hdr *goatorepo.RequestHeader) error { return nil },
		func(id string, reason error) {},
	)

	p.AddClient(clientAddress1, client1Rw)
	p.AddClient(clientAddress2, client2Rw)

	go p.Serve()

	go func() {

		client1Write <- &goatorepo.Rpc{
			Id: 1,
			Header: &goatorepo.RequestHeader{
				Source:      clientAddress1,
				Destination: clientAddress2,
			},
		}

		client1Write <- &goatorepo.Rpc{
			Id: 2,
			Header: &goatorepo.RequestHeader{
				Source:      clientAddress1,
				Destination: clientAddress2,
			},
		}

		client1Write <- &goatorepo.Rpc{
			Id: 3,
			Header: &goatorepo.RequestHeader{
				Source:      clientAddress1,
				Destination: clientAddress1,
			},
		}
	}()

	select {
	case outRpc := <-client1Read:
		require.Equal(t, 3, int(outRpc.Id))
	case <-time.After(2 * time.Second):
		t.FailNow()
	}
}
