package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	"github.com/avos-io/goat"
	rpcheader "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
)

func TestNew(t *testing.T) {
	new := NewServer()
	defer new.Stop()
	require.NotNil(t, new)
}

func TestStop(t *testing.T) {
	srv := NewServer()

	conn := testConn{}
	blockForever := make(chan readReturn)
	conn.On("Read", mock.Anything).Return(blockForever)

	done := make(chan struct{}, 1)
	go func() {
		srv.Serve(&conn)
		done <- struct{}{}
	}()

	srv.Stop()
	waitTimeout(t, done)
}

func TestUnary(t *testing.T) {
	t.Run("We can receive a unary RPC and send out its reply", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer()
		defer srv.Stop()

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/Unary"
		sent := testproto.Msg{Value: 42}
		exp := testproto.Msg{Value: 43}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(m *testproto.Msg) bool {
				return m.Value == sent.Value
			},
		)).Return(&exp, nil)

		testproto.RegisterTestServiceServer(srv, service)

		conn, readChan, writeChan := setupTestConn()

		go func() {
			srv.Serve(conn)
		}()

		readChan <- readReturn{wrapRpc(id, method, &sent), nil}

		select {
		case w := <-writeChan:
			is.Equal(id, w.GetId())
			is.Equal(method, w.GetHeader().GetMethod())
			is.NotNil(w.GetBody())
			is.Equal(exp.Value, unwrapBody(w).GetValue())
			is.NotNil(w.GetTrailer())
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on writeChan")
		}
	})

	t.Run("If the unary handler returns an error, we wrap that up", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer()
		defer srv.Stop()

		id := uint64(1)
		method := testproto.TestService_ServiceDesc.ServiceName + "/Unary"
		sent := testproto.Msg{Value: 42}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(m *testproto.Msg) bool {
				return m.Value == sent.Value
			},
		)).Return(nil, errTest)

		testproto.RegisterTestServiceServer(srv, service)

		conn, readChan, writeChan := setupTestConn()

		go func() {
			srv.Serve(conn)
		}()

		readChan <- readReturn{wrapRpc(id, method, &sent), nil}

		select {
		case w := <-writeChan:
			is.NotEqual(int32(codes.OK), w.GetStatus().GetCode())
			is.NotEmpty(w.GetStatus().GetMessage())
			is.Equal(id, w.GetId())
			is.Equal(method, w.GetHeader().GetMethod())
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on writeChan")
		}
	})
}

func TestServerStream(t *testing.T) {
	t.Run("We can fulfill a server stream request", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer()
		defer srv.Stop()

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/ServerStream"
		sent := testproto.Msg{Value: 1}

		expected := make([]*testproto.Msg, 10)
		for i := range expected {
			expected[i] = &testproto.Msg{Value: int32(i + 1)}
		}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().ServerStream(mock.Anything, mock.Anything).
			Run(
				func(m *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
					assert.Equal(t, sent.Value, m.Value)
					for _, exp := range expected {
						stream.Send(exp)
					}
				},
			).
			Return(nil)

		testproto.RegisterTestServiceServer(srv, service)

		conn, readChan, writeChan := setupTestConn()

		go func() {
			srv.Serve(conn)
		}()

		// Open stream
		readChan <- readReturn{&rpcheader.Rpc{
			Id: id,
			Header: &rpcheader.RequestHeader{
				Method: method,
			},
		}, nil}

		// SendMsg
		readChan <- readReturn{wrapRpc(id, method, &sent), nil}

		// CloseSend
		readChan <- readReturn{&rpcheader.Rpc{
			Id: id,
			Header: &rpcheader.RequestHeader{
				Method: method,
			},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.OK),
				Message: codes.OK.String(),
			},
			Trailer: &rpcheader.Trailer{},
		}, nil}

		// Read off replies
		exp := 1
		for {
			select {
			case got := <-writeChan:
				if exp <= len(expected) {
					is.Equal(int32(exp), unwrapBody(got).GetValue())
					exp++
				} else {
					is.NotNil(got.Trailer)
					return
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout")
			}
		}

	})
}

func waitTimeout(t *testing.T, on chan struct{}) {
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("TIMEOUT")
	case <-on:
		return
	}
}

func wrapRpc(id uint64, fullMethod string, msg *testproto.Msg) *rpcheader.Rpc {
	codec := encoding.GetCodec(proto.Name)

	body, err := codec.Marshal(msg)
	if err != nil {
		panic(err)
	}

	rpc := &rpcheader.Rpc{
		Id: id,
		Header: &rpcheader.RequestHeader{
			Method: fullMethod,
		},
		Body: &rpcheader.Body{Data: body},
	}
	return rpc
}

func unwrapBody(rpc *rpcheader.Rpc) *testproto.Msg {
	codec := encoding.GetCodec(proto.Name)

	if rpc.GetBody() == nil {
		return nil
	}

	var out testproto.Msg
	err := codec.Unmarshal(rpc.Body.Data, &out)
	if err != nil {
		panic(err)
	}
	return &out
}

type testConn struct {
	mock.Mock
}

type readReturn struct {
	rpc *rpcheader.Rpc
	err error
}

func (c *testConn) Read(ctx context.Context) (*rpcheader.Rpc, error) {
	args := c.Called(ctx)
	ch := args.Get(0).(chan readReturn)
	select {
	case rr := <-ch:
		return rr.rpc, rr.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *testConn) Write(ctx context.Context, rpc *rpcheader.Rpc) error {
	args := c.Called(ctx, rpc)
	err := args.Error(0)
	return err
}

func setupTestConn() (goat.RpcReadWriter, chan readReturn, chan *rpcheader.Rpc) {
	conn := testConn{}

	readChan := make(chan readReturn)
	conn.On("Read", mock.Anything).Return(readChan)

	writeChan := make(chan *rpcheader.Rpc)
	conn.On("Write", mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			got, _ := args.Get(1).(*rpcheader.Rpc)
			writeChan <- got
		})

	return &conn, readChan, writeChan
}
