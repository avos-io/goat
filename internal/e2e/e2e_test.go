// package e2e contains end-to-end tests
package e2e

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/avos-io/goat/pkg/client"
	"github.com/avos-io/goat/pkg/server"
)

var errTest = errors.New("TEST ERROR (EXPECTED)")

func TestUnary(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		send := &testproto.Msg{Value: 42}
		exp := &testproto.Msg{Value: 9001}

		ms := mocks.NewTestServiceServer(t)
		ms.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(msg *testproto.Msg) bool {
				return msg.GetValue() == send.GetValue()
			},
		)).Return(exp, nil)

		client, ctx, teardown := setup(ms)
		defer teardown()

		reply, err := client.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply)
		is.Equal(exp.GetValue(), reply.GetValue())
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		ms := mocks.NewTestServiceServer(t)
		ms.EXPECT().Unary(mock.Anything, mock.Anything).Return(nil, errTest)

		client, ctx, teardown := setup(ms)
		defer teardown()

		reply, err := client.Unary(ctx, &testproto.Msg{Value: 4})
		is.Error(err)
		is.Nil(reply)
	})
}

func TestServerStream(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		sent := &testproto.Msg{Value: 10}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().ServerStream(mock.Anything, mock.Anything).
			Run(
				func(msg *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
					v := msg.GetValue()
					assert.Equal(t, sent.GetValue(), v)
					for i := 1; i < int(v); i++ {
						stream.Send(&testproto.Msg{Value: int32(i)})
					}
				},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.ServerStream(ctx, sent)
		is.NoError(err)
		is.NotNil(stream)

		exp := int32(1)
		for {
			recv, err := stream.Recv()
			if err == io.EOF {
				is.Equal(exp, sent.GetValue())
				return
			}
			is.Equal(exp, recv.GetValue())
			exp++
		}
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().ServerStream(mock.Anything, mock.Anything).Return(errTest)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.ServerStream(ctx, &testproto.Msg{Value: 42})
		is.NoError(err)
		is.NotNil(stream)

		recv, err := stream.Recv()
		is.Error(err)
		is.Nil(recv)
	})
}

func TestClientStream(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().ClientStream(mock.Anything).
			Run(func(stream testproto.TestService_ClientStreamServer) {
				sum := int32(0)
				for {
					msg, err := stream.Recv()
					if err == io.EOF {
						stream.SendAndClose(&testproto.Msg{Value: sum})
						return
					}
					is.NoError(err)
					sum += msg.GetValue()
				}
			},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.ClientStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		sum := 0
		for i := 1; i < 10; i++ {
			err = stream.Send(&testproto.Msg{Value: int32(i)})
			is.NoError(err)
			sum += i
		}

		reply, err := stream.CloseAndRecv()
		is.NoError(err)
		is.Equal(int32(sum), reply.GetValue())
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().ClientStream(mock.Anything).Return(errTest)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.ClientStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		err = stream.Send(&testproto.Msg{Value: int32(1)})
		is.NoError(err)

		msg, err := stream.CloseAndRecv()
		is.Error(err)
		is.Nil(msg)
	})
}

func TestBidiStream(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).
			Run(
				func(stream testproto.TestService_BidiStreamServer) {
					for {
						msg, err := stream.Recv()
						if err == io.EOF {
							return
						}
						is.NoError(err)
						err = stream.Send(&testproto.Msg{Value: msg.GetValue()})
						is.NoError(err)
					}
				},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.BidiStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		for i := 0; i < 10; i++ {
			stream.Send(&testproto.Msg{Value: int32(i)})
			reply, err := stream.Recv()
			is.NoError(err)
			is.Equal(int32(i), reply.GetValue())
		}
		is.NoError(stream.CloseSend())
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).Return(errTest)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.BidiStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		stream.Send(&testproto.Msg{Value: int32(0)})
		reply, err := stream.Recv()
		is.Error(err)
		is.Nil(reply)
	})
}

func setup(s testproto.TestServiceServer) (testproto.TestServiceClient, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	serverConn := testutil.NewTestConn()
	clientConn := testutil.NewTestConn()

	// server output -> client input
	go func() {
		for {
			select {
			case <-ctx.Done():
				clientConn.ReadChan <- testutil.ReadReturn{Rpc: nil, Err: io.EOF}
				return
			case w := <-serverConn.WriteChan:
				clientConn.ReadChan <- testutil.ReadReturn{Rpc: w, Err: nil}
			}
		}
	}()

	// client output -> server input
	go func() {
		for {
			select {
			case <-ctx.Done():
				serverConn.ReadChan <- testutil.ReadReturn{Rpc: nil, Err: io.EOF}
				return
			case w := <-clientConn.WriteChan:
				serverConn.ReadChan <- testutil.ReadReturn{Rpc: w, Err: nil}
			}
		}
	}()

	server := server.NewServer()
	testproto.RegisterTestServiceServer(server, s)

	go func() {
		server.Serve(serverConn)
	}()

	client := testproto.NewTestServiceClient(
		client.NewClientConn(clientConn),
	)

	teardown := func() {
		cancel()
		server.Stop()
	}
	return client, ctx, teardown
}
