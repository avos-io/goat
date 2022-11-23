// package goat_test contains end-to-end tests
package goat_test

import (
	"context"
	"errors"
	"io"
	"testing"

	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/avos-io/goat/client"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/avos-io/goat/server"
)

var errTest = errors.New("TEST ERROR (EXPECTED)")

func TestUnary(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		send := &testproto.Msg{Value: 42}
		exp := &testproto.Msg{Value: 9001}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(msg *testproto.Msg) bool {
				return msg.GetValue() == send.GetValue()
			},
		)).Return(exp, nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		reply, err := client.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply)
		is.Equal(exp.GetValue(), reply.GetValue())
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.Anything).Return(nil, errTest)

		client, ctx, teardown := setup(service)
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

		// Setting up the server stream is a multi-part process (open, send initial
		// message, send trailer) and depending on when we receive the server error,
		// we'll either get an error on client.ServerStream or an error on first
		// stream.Recv()
		if err != nil {
			is.Nil(stream)
		} else {
			is.NotNil(stream)
			recv, err := stream.Recv()
			is.Error(err)
			is.Nil(recv)
		}
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
			}).
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

		msg, err := stream.CloseAndRecv()
		is.Error(err)
		is.Nil(msg)
	})
}

func TestBidiStream(t *testing.T) {
	t.Run("OK: client messages first", func(t *testing.T) {
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

	t.Run("OK: server messages first", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).
			Run(
				func(stream testproto.TestService_BidiStreamServer) {
					for i := 0; i < 10; i++ {
						err := stream.Send(&testproto.Msg{Value: int32(i)})
						is.NoError(err)

						msg, err := stream.Recv()
						is.NoError(err)
						is.NotNil(msg.GetValue())
					}
				},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.BidiStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		for {
			got, err := stream.Recv()
			if err == io.EOF {
				return
			}
			is.NoError(err)
			is.NotNil(got)
			err = stream.Send(&testproto.Msg{Value: got.GetValue()})
			is.NoError(err)
		}
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

func TestStreamHeaders(t *testing.T) {
	t.Run("Headers sent with SendHeader", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).
			Run(
				func(stream testproto.TestService_BidiStreamServer) {
					stream.SetHeader(metadata.New(map[string]string{"one": "1"}))
					stream.SetHeader(metadata.New(map[string]string{"two": "2"}))
					stream.SendHeader(metadata.New(map[string]string{"three": "3"}))

					msg, err := stream.Recv()
					is.NoError(err)
					is.NotNil(msg)
					err = stream.Send(&testproto.Msg{Value: msg.GetValue()})
					is.NoError(err)
				},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.BidiStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		md, err := stream.Header()
		is.NoError(err)

		is.Len(md, 3)
		is.Equal("1", md["one"][0])
		is.Equal("2", md["two"][0])
		is.Equal("3", md["three"][0])

		stream.Send(&testproto.Msg{Value: int32(0)})
		stream.Recv()
	})

	t.Run("Headers sent on first message", func(t *testing.T) {
		is := require.New(t)

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).
			Run(
				func(stream testproto.TestService_BidiStreamServer) {
					stream.SetHeader(metadata.New(map[string]string{"one": "1"}))
					stream.SetHeader(metadata.New(map[string]string{"two": "2"}))
					stream.SetHeader(metadata.New(map[string]string{"three": "3"}))

					err := stream.Send(&testproto.Msg{Value: 13})
					is.NoError(err)
				},
			).
			Return(nil)

		client, ctx, teardown := setup(service)
		defer teardown()

		stream, err := client.BidiStream(ctx)
		is.NoError(err)
		is.NotNil(stream)

		md, err := stream.Header()
		is.NoError(err)

		is.Len(md, 3)
		is.Equal("1", md["one"][0])
		is.Equal("2", md["two"][0])
		is.Equal("3", md["three"][0])

		stream.Recv()
	})
}

func TestStreamTrailers(t *testing.T) {
	is := require.New(t)

	service := mocks.NewTestServiceServer(t)
	service.EXPECT().ClientStream(mock.Anything).
		Run(func(stream testproto.TestService_ClientStreamServer) {
			md := metadata.New(map[string]string{"foo": "bar"})
			stream.SetTrailer(md)
			_, err := stream.Recv()
			if err == io.EOF {
				stream.SendAndClose(&testproto.Msg{Value: 1})
				return
			}
			is.NoError(err)
		}).
		Return(nil)

	client, ctx, teardown := setup(service)
	defer teardown()

	stream, err := client.ClientStream(ctx)
	is.NoError(err)
	is.NotNil(stream)

	err = stream.Send(&testproto.Msg{Value: int32(1)})
	is.NoError(err)

	stream.CloseAndRecv()

	trailer := stream.Trailer()
	is.NotNil(trailer)
	is.Equal(trailer["foo"][0], "bar")
}

func TestUnaryInterceptor(t *testing.T) {
	is := require.New(t)

	clientInterceptor := func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "foo", "1")
		return invoker(ctx, method, req, reply, cc, opts...)
	}

	serverInterceptor := func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		is.True(ok)

		foo, ok := md["foo"]
		is.True(ok)
		is.Len(foo, 1)
		is.Equal("1", foo[0])

		md.Append("bar", "2")
		newCtx := metadata.NewIncomingContext(ctx, md)
		return handler(newCtx, req)
	}

	service := mocks.NewTestServiceServer(t)
	service.EXPECT().Unary(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, msg *testproto.Msg) {
			md, ok := metadata.FromIncomingContext(ctx)
			is.True(ok)

			foo, ok := md["foo"]
			is.True(ok)
			is.Len(foo, 1)
			is.Equal("1", foo[0])

			bar, ok := md["bar"]
			is.True(ok)
			is.Len(bar, 1)
			is.Equal("2", bar[0])
		}).
		Return(&testproto.Msg{Value: 1}, nil)

	client, ctx, teardown := setupOpts(
		service,
		[]client.DialOption{
			client.WithUnaryInterceptor(clientInterceptor),
		},
		[]server.ServerOption{
			server.UnaryInterceptor(serverInterceptor),
		},
	)
	defer teardown()

	reply, err := client.Unary(ctx, &testproto.Msg{Value: 42})
	is.NoError(err)
	is.NotNil(reply)
}

func TestChainUnaryInterceptor(t *testing.T) {
	is := require.New(t)

	makeClientInterceptor := func(k, v string) grpc.UnaryClientInterceptor {
		return func(
			ctx context.Context,
			method string,
			req, reply interface{},
			cc *grpc.ClientConn,
			invoker grpc.UnaryInvoker,
			opts ...grpc.CallOption,
		) error {
			ctx = metadata.AppendToOutgoingContext(ctx, k, v)
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}

	clientMd := map[string]string{
		"one":   "1",
		"two":   "2",
		"three": "3",
	}

	clientInterceptors := []grpc.UnaryClientInterceptor{}
	for k, v := range clientMd {
		clientInterceptors = append(clientInterceptors, makeClientInterceptor(k, v))
	}

	makeServerInterceptor := func(k, v string) grpc.UnaryServerInterceptor {
		return func(
			ctx context.Context,
			req interface{},
			info *grpc.UnaryServerInfo,
			handler grpc.UnaryHandler,
		) (interface{}, error) {
			md, ok := metadata.FromIncomingContext(ctx)
			is.True(ok)

			for ek, ev := range clientMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}

			md.Append(k, v)
			newCtx := metadata.NewIncomingContext(ctx, md)
			return handler(newCtx, req)
		}
	}

	serverMd := map[string]string{
		"ein":  "1",
		"zwei": "2",
		"drei": "3",
	}

	serverInterceptors := []grpc.UnaryServerInterceptor{}
	for k, v := range serverMd {
		serverInterceptors = append(serverInterceptors, makeServerInterceptor(k, v))
	}

	service := mocks.NewTestServiceServer(t)
	service.EXPECT().Unary(mock.Anything, mock.Anything).
		Run(func(ctx context.Context, msg *testproto.Msg) {
			md, ok := metadata.FromIncomingContext(ctx)
			is.True(ok)

			for ek, ev := range clientMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}

			for ek, ev := range serverMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}
		}).
		Return(&testproto.Msg{Value: 1}, nil)

	client, ctx, teardown := setupOpts(
		service,
		[]client.DialOption{
			client.WithChainUnaryInterceptor(clientInterceptors...),
		},
		[]server.ServerOption{
			server.UnaryInterceptor(
				grpcMiddleware.ChainUnaryServer(serverInterceptors...),
			),
		},
	)
	defer teardown()

	reply, err := client.Unary(ctx, &testproto.Msg{Value: 42})
	is.NoError(err)
	is.NotNil(reply)
}

func TestStreamInterceptor(t *testing.T) {
	is := require.New(t)

	clientInterceptor := func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, "foo", "1")
		return streamer(ctx, desc, cc, method, opts...)
	}

	serverInterceptor := func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := stream.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		is.True(ok)

		foo, ok := md["foo"]
		is.True(ok)
		is.Len(foo, 1)
		is.Equal("1", foo[0])

		md.Append("bar", "2")
		newCtx := metadata.NewIncomingContext(ctx, md)

		return handler(srv, &grpcMiddleware.WrappedServerStream{
			ServerStream:   stream,
			WrappedContext: newCtx,
		})
	}

	service := mocks.NewTestServiceServer(t)
	service.EXPECT().ServerStream(mock.Anything, mock.Anything).
		Run(func(_ *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
			md, ok := metadata.FromIncomingContext(stream.Context())
			is.True(ok)

			foo, ok := md["foo"]
			is.True(ok)
			is.Len(foo, 1)
			is.Equal("1", foo[0])

			bar, ok := md["bar"]
			is.True(ok)
			is.Len(bar, 1)
			is.Equal("2", bar[0])

			stream.Send(&testproto.Msg{Value: 42})
		}).
		Return(nil)

	client, ctx, teardown := setupOpts(
		service,
		[]client.DialOption{
			client.WithStreamInterceptor(clientInterceptor),
		},
		[]server.ServerOption{
			server.StreamInterceptor(serverInterceptor),
		},
	)
	defer teardown()

	stream, err := client.ServerStream(ctx, &testproto.Msg{Value: 42})
	is.NoError(err)
	is.NotNil(stream)

	stream.Recv()
}

func TestChainStreamInterceptor(t *testing.T) {
	is := require.New(t)

	makeClientInterceptor := func(k, v string) grpc.StreamClientInterceptor {
		return func(
			ctx context.Context,
			desc *grpc.StreamDesc,
			cc *grpc.ClientConn,
			method string,
			streamer grpc.Streamer,
			opts ...grpc.CallOption,
		) (grpc.ClientStream, error) {
			ctx = metadata.AppendToOutgoingContext(ctx, k, v)
			return streamer(ctx, desc, cc, method, opts...)
		}
	}

	clientMd := map[string]string{
		"one":   "1",
		"two":   "2",
		"three": "3",
	}

	clientInterceptors := []grpc.StreamClientInterceptor{}
	for k, v := range clientMd {
		clientInterceptors = append(clientInterceptors, makeClientInterceptor(k, v))
	}

	makeServerInterceptor := func(k, v string) grpc.StreamServerInterceptor {
		return func(
			srv interface{},
			stream grpc.ServerStream,
			info *grpc.StreamServerInfo,
			handler grpc.StreamHandler,
		) error {
			ctx := stream.Context()
			md, ok := metadata.FromIncomingContext(ctx)
			is.True(ok)

			for ek, ev := range clientMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}

			md.Append(k, v)
			newCtx := metadata.NewIncomingContext(ctx, md)

			return handler(srv, &grpcMiddleware.WrappedServerStream{
				ServerStream:   stream,
				WrappedContext: newCtx,
			})
		}
	}

	serverMd := map[string]string{
		"ein":  "1",
		"zwei": "2",
		"drei": "3",
	}

	serverInterceptors := []grpc.StreamServerInterceptor{}
	for k, v := range serverMd {
		serverInterceptors = append(serverInterceptors, makeServerInterceptor(k, v))
	}

	service := mocks.NewTestServiceServer(t)
	service.EXPECT().ServerStream(mock.Anything, mock.Anything).
		Run(func(_ *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
			md, ok := metadata.FromIncomingContext(stream.Context())
			is.True(ok)

			for ek, ev := range clientMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}

			for ek, ev := range serverMd {
				match, ok := md[ek]
				is.True(ok)
				is.Equal(ev, match[0])
			}

			stream.Send(&testproto.Msg{Value: 42})
		}).
		Return(nil)

	client, ctx, teardown := setupOpts(
		service,
		[]client.DialOption{
			client.WithChainStreamInterceptor(clientInterceptors...),
		},
		[]server.ServerOption{
			server.StreamInterceptor(
				grpcMiddleware.ChainStreamServer(serverInterceptors...),
			),
		},
	)
	defer teardown()

	stream, err := client.ServerStream(ctx, &testproto.Msg{Value: 42})
	is.NoError(err)
	is.NotNil(stream)

	stream.Recv()
}

func setup(s testproto.TestServiceServer) (testproto.TestServiceClient, context.Context, func()) {
	return setupOpts(s, nil, nil)
}

func setupOpts(
	s testproto.TestServiceServer,
	clientOpts []client.DialOption,
	serverOpts []server.ServerOption,
) (testproto.TestServiceClient, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	serverConn := testutil.NewTestConn()
	clientConn := testutil.NewTestConn()

	// server output -> client input
	go func() {
		for {
			select {
			case <-ctx.Done():
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
				return
			case w := <-clientConn.WriteChan:
				serverConn.ReadChan <- testutil.ReadReturn{Rpc: w, Err: nil}
			}
		}
	}()

	server := server.NewServer(serverOpts...)
	testproto.RegisterTestServiceServer(server, s)

	go func() {
		server.Serve(serverConn)
	}()

	client := testproto.NewTestServiceClient(
		client.NewClientConn(clientConn, clientOpts...),
	)

	teardown := func() {
		cancel()
		server.Stop()
	}
	return client, ctx, teardown
}
