package main

// ref: https://github.com/improbable-eng/grpc-web/pull/1084/files
// ref: https://github.com/fullstorydev/grpchan/blob/9b5ad76b6f3d9146862d71690ea87eb9435acc4e/httpgrpc/io.go#L119

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/avos-io/grpc-websockets/internal/client"
	"github.com/avos-io/grpc-websockets/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	grpcMiddleware "github.com/grpc-ecosystem/go-grpc-middleware"

	"nhooyr.io/websocket"
)

type TestService struct {
	rpcheader.UnsafeIntServiceServer
}

func (*TestService) Add(
	ctx context.Context,
	req *rpcheader.AddRequest,
) (*rpcheader.IntMessage, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	for k, vs := range md {
		for _, v := range vs {
			log.Printf("Add.metadata: %s=%s", k, v)
		}
	}
	return &rpcheader.IntMessage{
		Value: 1,
	}, nil
	// return nil, fmt.Errorf("hello world")
}

func (*TestService) BidiStream(stream rpcheader.IntService_BidiStreamServer) error {
	for i := 0; i < 10; i++ {
		rcv, err := stream.Recv()
		if err == io.EOF {
			log.Printf("BidiStream EOF")
			return nil
		}
		if err != nil {
			log.Printf("BidiStream error: %v", err)
			return err
		}
		log.Printf("BidiStream recv: %d", rcv.GetValue())
		err = stream.Send(&rpcheader.IntMessage{Value: rcv.GetValue() + 1})
		if err != nil {
			log.Printf("BidiStream send error: %v", err)
			return err
		}
	}
	return nil
}

func (*TestService) ServerStream(
	msg *rpcheader.IntMessage,
	stream rpcheader.IntService_ServerStreamServer,
) error {

	md, _ := metadata.FromIncomingContext(stream.Context())
	for k, vs := range md {
		for _, v := range vs {
			log.Printf("ServerStream.metadata: %s=%s", k, v)
		}
	}

	v := msg.GetValue()
	log.Printf("SerServerStream recv %d", v)
	stream.SetHeader(metadata.New(map[string]string{
		"test":  "hello",
		"test2": "world",
	}))
	for i := 0; i < 4; i++ {
		log.Printf("ServerStream send %d", v+int32(i))
		stream.Send(&rpcheader.IntMessage{Value: v + int32(i)})
	}
	return nil
}

func (*TestService) ClientStream(stream rpcheader.IntService_ClientStreamServer) error {
	for {
		m, err := stream.Recv()
		if err == io.EOF {
			log.Printf("ClientStream: EOF")
			return stream.SendAndClose(&rpcheader.IntMessage{Value: 9001})
		}
		if err != nil {
			log.Printf("ClientStream error: %v", err)
			return err
		}
		log.Printf("ClientStream recv: %d", m.GetValue())
	}
}

func main() {
	interceptor := NewInterceptor()

	srv := server.NewServer(
		server.UnaryInterceptor(interceptor.Unary()),
		server.StreamInterceptor(interceptor.Stream()),
	)

	ts := &TestService{}

	rpcheader.RegisterIntServiceServer(srv, ts)

	// Server
	if true {
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			defer c.Close(websocket.StatusInternalError, "the sky is falling (server)")

			_, cancel := context.WithTimeout(r.Context(), time.Second*10)
			defer cancel()

			srv.ServeWebsocket(c)

			c.Close(websocket.StatusNormalClosure, "")
		})

		listener, err := net.Listen("tcp", ":8080")

		go func() {
			err = http.Serve(listener, fn)
			log.Fatal(err)
		}()
	}

	// Client
	if true {
		c, _, err := websocket.Dial(context.Background(), "ws://localhost:8080", nil)
		if err != nil {
			panic(err)
		}
		defer c.Close(websocket.StatusInternalError, "the sky is falling (client)")

		cci := client.NewWebsocketClientConn(
			c,
			client.WithUnaryInterceptor(unaryInterceptAddMetadata),
			client.WithStreamInterceptor(streamInterceptAddMetadata),
		)
		isc := rpcheader.NewIntServiceClient(cci)

		doUnary(isc)
		doServerStream(isc, 1)
		doClientStream(isc, 100)
		doBidiStream(isc)

		log.Printf("FIN")

		c.Close(websocket.StatusNormalClosure, "")
	}
}

func doUnary(isc rpcheader.IntServiceClient) {
	log.Printf("doUnary----")
	resp, err := isc.Add(context.Background(), &rpcheader.AddRequest{A: 1, B: 2})
	log.Printf("Add returns: %v || %v\n", resp, err)
}

func doClientStream(isc rpcheader.IntServiceClient, v int32) {
	log.Printf("doClientStream----")
	stream, err := isc.ClientStream(context.Background())
	if err != nil {
		log.Panicf("failed to setup client stream")
	}
	end := v + 4
	for i := v; i < end; i++ {
		stream.Send(&rpcheader.IntMessage{Value: int32(i)})
	}
	reply, err := stream.CloseAndRecv()
	if err != nil {
		return
	}
	log.Printf("doClientStream got %d", reply.GetValue())
}

func doServerStream(isc rpcheader.IntServiceClient, v int32) {
	log.Printf("doServerStream----")
	client, err := isc.ServerStream(context.Background(), &rpcheader.IntMessage{Value: v})
	if err != nil {
		log.Panicf("failed to open stream: %s", err.Error())
	}
	header, err := client.Header()
	if err != nil {
		log.Panicf("error reading server stream header: %v", err)
	}
	log.Printf("Got header: %v", header)
	for {
		reply, err := client.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("stream recv error: %s", err.Error())
			break
		}
		log.Printf("doServerStream got %d", reply.GetValue())
	}
}

func doBidiStream(isc rpcheader.IntServiceClient) {
	log.Printf("doBidiStream----")
	stream, err := isc.BidiStream(context.Background())
	if err != nil {
		log.Panicf("failed to start bidi stream, %v", err)
	}
	waitc := make(chan struct{})
	go func() {
		for {
			reply, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}
			if err != nil {
				log.Fatalf("doBidiStream: failed to receive: %v", err)
			}
			log.Printf("doBidiStream got: %d", reply.GetValue())
		}
	}()
	for i := 0; i < 10; i++ {
		if err := stream.Send(&rpcheader.IntMessage{Value: int32(i)}); err != nil {
			log.Fatalf("doBidiStream failed to send: %v", err)
		}
	}
	stream.CloseSend()
	<-waitc
}

type Interceptor struct{}

func NewInterceptor() Interceptor {
	return Interceptor{}
}

// Unary is the interceptor for authenticating unary gRPC calls.
func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		newCtx, err := i.check(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// Stream is the interceptor for authenticating streaming gRPC calls.
func (i *Interceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		newCtx, err := i.check(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, &grpcMiddleware.WrappedServerStream{
			ServerStream:   stream,
			WrappedContext: newCtx,
		})
	}
}

func (i *Interceptor) check(
	ctx context.Context,
	fullMethod string,
) (context.Context, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
	}

	/*
		// Grab the foo
		foo, ok := md["foo"]
		if !ok || len(foo) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing foo")
		}
	*/

	// Inject the bar
	md.Append("bar", "baaaaarrrrr")
	newCtx := metadata.NewIncomingContext(ctx, md)
	return newCtx, nil
}

func unaryInterceptAddMetadata(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	return invoker(addMetadata(ctx), method, req, reply, cc, opts...)
}

func streamInterceptAddMetadata(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return streamer(addMetadata(ctx), desc, cc, method, opts...)
}

func addMetadata(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		"foo-bar", "boo-far",
	)
}
