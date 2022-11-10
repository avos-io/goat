package client

import (
	"context"
	"fmt"
	"log"
	"time"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/avos-io/grpc-websockets/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"nhooyr.io/websocket"
)

type WebsocketClientConn struct {
	mp *RpcMultiplexer

	unaryInterceptor        grpc.UnaryClientInterceptor
	chainUnaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptor       grpc.StreamClientInterceptor
	chainStreamInterceptors []grpc.StreamClientInterceptor
}

func NewWebsocketClientConn(
	conn *websocket.Conn,
	opts ...DialOption,
) grpc.ClientConnInterface {
	cc := WebsocketClientConn{
		mp: NewRpcMultiplexer(conn),
	}
	for _, opt := range opts {
		opt.apply(&cc)
	}

	cc.unaryInterceptor = chainedUnaryInterceptors(&cc)
	cc.streamInterceptor = chainedStreamInterceptors(&cc)

	return &cc
}

// Invoke performs a unary RPC and returns after the response is received
// into reply.
func (wcc *WebsocketClientConn) Invoke(
	ctx context.Context,
	method string,
	args interface{},
	reply interface{},
	opts ...grpc.CallOption,
) error {
	if wcc.unaryInterceptor != nil {
		return wcc.unaryInterceptor(ctx, method, args, reply, nil, wcc.asInvoker, opts...)
	}
	grpc.WithChainUnaryInterceptor()
	return wcc.invoke(ctx, method, args, reply, opts...)
}

func (wcc *WebsocketClientConn) invoke(
	ctx context.Context,
	method string,
	args interface{},
	reply interface{},
	opts ...grpc.CallOption,
) error {
	if len(opts) > 0 {
		log.Panic("opts unsupported")
	}

	headers := headersFromContext(ctx)

	codec := encoding.GetCodec(proto.Name)
	body, err := codec.Marshal(args)
	if err != nil {
		return err
	}

	replyBody, err := wcc.mp.CallUnaryMethod(ctx,
		&rpcheader.RequestHeader{
			Method:  method,
			Headers: headers,
		},
		&rpcheader.Body{
			Data: body,
		},
	)

	if err != nil {
		return err
	}

	return codec.Unmarshal(replyBody.Data, reply)
}

func (wcc *WebsocketClientConn) asInvoker(
	ctx context.Context,
	method string,
	req, reply interface{},
	_ *grpc.ClientConn,
	opts ...grpc.CallOption,
) error {
	return wcc.invoke(ctx, method, req, reply, opts...)
}

// NewStream begins a streaming RPC.
func (wcc *WebsocketClientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if wcc.streamInterceptor != nil {
		return wcc.streamInterceptor(ctx, desc, nil, method, wcc.asStreamer, opts...)
	}
	return wcc.newStream(ctx, desc, method, opts...)
}

func (wcc *WebsocketClientConn) newStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if len(opts) > 0 {
		log.Panic("opts unsupported")
	}

	headers := headersFromContext(ctx)

	header := rpcheader.RequestHeader{
		Method:  method,
		Headers: headers,
	}

	stream, err := wcc.mp.NewStream(ctx, &header)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (wcc *WebsocketClientConn) asStreamer(
	ctx context.Context,
	desc *grpc.StreamDesc,
	_ *grpc.ClientConn,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return wcc.newStream(ctx, desc, method, opts...)
}

// headersFromContext returns request headers to send to the remote host based
// on the specified context. GRPC clients store outgoing metadata into the
// context, which is translated into headers. Also, a context deadline will be
// propagated to the server via GRPC timeout metadata.
func headersFromContext(ctx context.Context) []*rpcheader.KeyValue {
	h := []*rpcheader.KeyValue{}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		h = append(h, internal.ToKeyValue(md)...)
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		ms := int64(timeout / time.Millisecond)
		if ms <= 0 {
			ms = 1
		}
		h = append(h, &rpcheader.KeyValue{
			Key:   "GRPC-Timeout",
			Value: fmt.Sprintf("%dm", ms),
		})
	}
	return h
}

// chainedUnaryInterceptors chains all unary client interceptors into one.
func chainedUnaryInterceptors(wcc *WebsocketClientConn) grpc.UnaryClientInterceptor {
	interceptors := wcc.chainUnaryInterceptors
	if wcc.unaryInterceptor != nil {
		interceptors = append(
			[]grpc.UnaryClientInterceptor{wcc.unaryInterceptor},
			interceptors...,
		)
	}
	switch len(interceptors) {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	default:
		return func(
			ctx context.Context,
			method string,
			req interface{},
			reply interface{},
			cc *grpc.ClientConn,
			invoker grpc.UnaryInvoker,
			opts ...grpc.CallOption,
		) error {
			return interceptors[0](
				ctx,
				method,
				req,
				reply,
				cc,
				getChainUnaryInvoker(interceptors, 0, invoker),
				opts...,
			)
		}
	}
}

// getChainUnaryInvoker recursively generates the chained unary invoker.
func getChainUnaryInvoker(
	interceptors []grpc.UnaryClientInterceptor,
	curr int,
	finalInvoker grpc.UnaryInvoker,
) grpc.UnaryInvoker {
	if curr == len(interceptors)-1 {
		return finalInvoker
	}
	return func(
		ctx context.Context,
		method string,
		req interface{},
		reply interface{},
		cc *grpc.ClientConn,
		opts ...grpc.CallOption,
	) error {
		return interceptors[curr+1](
			ctx,
			method,
			req,
			reply,
			cc,
			getChainUnaryInvoker(interceptors, curr+1, finalInvoker),
			opts...,
		)
	}
}

// chainedStreamInterceptors chains all stream client interceptors into one.
func chainedStreamInterceptors(wcc *WebsocketClientConn) grpc.StreamClientInterceptor {
	interceptors := wcc.chainStreamInterceptors
	if wcc.streamInterceptor != nil {
		interceptors = append(
			[]grpc.StreamClientInterceptor{wcc.streamInterceptor},
			interceptors...)
	}
	switch len(interceptors) {
	case 0:
		return nil
	case 1:
		return interceptors[0]
	default:
		return func(
			ctx context.Context,
			desc *grpc.StreamDesc,
			cc *grpc.ClientConn,
			method string,
			streamer grpc.Streamer,
			opts ...grpc.CallOption,
		) (grpc.ClientStream, error) {
			return interceptors[0](
				ctx,
				desc,
				cc,
				method,
				getChainStreamer(interceptors, 0, streamer),
				opts...)
		}
	}
}

// getChainStreamer recursively generates the chained client stream constructor.
func getChainStreamer(
	interceptors []grpc.StreamClientInterceptor,
	curr int,
	finalStreamer grpc.Streamer,
) grpc.Streamer {
	if curr == len(interceptors)-1 {
		return finalStreamer
	}
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		return interceptors[curr+1](
			ctx,
			desc,
			cc,
			method,
			getChainStreamer(interceptors, curr+1, finalStreamer),
			opts...,
		)
	}
}
