package sam

// An end to end test of the following topology:
//
// [Client] <-> [Proxy] <-> [Server]

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/avos-io/goat"
	"github.com/avos-io/goat/client"
	wrapped "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/server"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type goatOverPipe struct {
	conn       net.Conn
	readMutex  sync.Mutex
	writeMutex sync.Mutex
}

func newGoatOverPipe(c net.Conn) *goatOverPipe {
	return &goatOverPipe{conn: c}
}

func (g *goatOverPipe) Read(ctx context.Context) (*wrapped.Rpc, error) {
	g.readMutex.Lock()
	defer g.readMutex.Unlock()

	// XXX: no timeout support here
	var msgSize uint32
	err := binary.Read(g.conn, binary.BigEndian, &msgSize)
	if err != nil {
		panic(err)
	}

	data := make([]byte, msgSize)
	_, err = io.ReadFull(g.conn, data)
	if err != nil {
		panic(err)
	}

	var rpc wrapped.Rpc
	err = proto.Unmarshal(data, &rpc)
	if err != nil {
		panic(err)
	}

	return &rpc, nil
}

func (g *goatOverPipe) Write(ctx context.Context, pkt *wrapped.Rpc) error {
	g.writeMutex.Lock()
	defer g.writeMutex.Unlock()

	data, err := proto.Marshal(pkt)
	if err != nil {
		panic(err)
	}

	var msgSize uint32 = uint32(len(data))
	err = binary.Write(g.conn, binary.BigEndian, &msgSize)
	if err != nil {
		panic(err)
	}

	_, err = g.conn.Write(data)
	if err != nil {
		panic(err)
	}

	return nil
}

type simulatedServer struct {
	srv *server.Server
}

func newSimulatedServer(transport goat.RpcReadWriter) *simulatedServer {
	srv := server.NewServer()

	go func() {
		err := srv.Serve(transport)
		panic(err)
	}()

	return &simulatedServer{srv}
}

type simulatedProxy struct {
	t1, t2 goat.RpcReadWriter
}

func newSimulatedProxy(t1, t2 goat.RpcReadWriter) *simulatedProxy {
	proxy := &simulatedProxy{t1, t2}

	go proxy.forward(t1, t2)
	go proxy.forward(t2, t1)

	return proxy
}

func (p *simulatedProxy) forward(from, to goat.RpcReadWriter) {
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

type simulatedClient struct {
	transport goat.RpcReadWriter
}

func newSimulatedClient(transport goat.RpcReadWriter) *simulatedClient {
	return &simulatedClient{transport}
}

func (c *simulatedClient) newClientConn() grpc.ClientConnInterface {
	return client.NewClientConn(c.transport)
}

type echoServer struct {
	testproto.UnimplementedTestServiceServer
}

func (*echoServer) Unary(_ context.Context, m *testproto.Msg) (*testproto.Msg, error) {
	return m, nil
}

func TestSam(t *testing.T) {
	// Make a pipe for communication between Proxy and Server
	ps1, ps2 := net.Pipe()
	// And between Client and Proxy
	ps3, ps4 := net.Pipe()

	simServer := newSimulatedServer(newGoatOverPipe(ps1))

	echoServer := &echoServer{}

	testproto.RegisterTestServiceServer(simServer.srv, echoServer)

	_ = newSimulatedProxy(newGoatOverPipe(ps2), newGoatOverPipe(ps3))

	simClient := newSimulatedClient(newGoatOverPipe(ps4))

	tpClient := testproto.NewTestServiceClient(simClient.newClientConn())
	result, err := tpClient.Unary(context.Background(), &testproto.Msg{Value: 11})
	if err != nil {
		panic(err)
	}

	require.Equal(t, int32(11), result.Value)
}
