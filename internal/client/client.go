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
)

type ClientConn struct {
	mp *RpcMultiplexer

	unaryInterceptor        grpc.UnaryClientInterceptor
	chainUnaryInterceptors  []grpc.UnaryClientInterceptor
	streamInterceptor       grpc.StreamClientInterceptor
	chainStreamInterceptors []grpc.StreamClientInterceptor
}

func NewClientConn(conn internal.RpcReadWriter, opts ...DialOption) grpc.ClientConnInterface {
	cc := ClientConn{
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
func (cc *ClientConn) Invoke(
	ctx context.Context,
	method string,
	args interface{},
	reply interface{},
	opts ...grpc.CallOption,
) error {
	if cc.unaryInterceptor != nil {
		return cc.unaryInterceptor(ctx, method, args, reply, nil, cc.asInvoker, opts...)
	}
	grpc.WithChainUnaryInterceptor()
	return cc.invoke(ctx, method, args, reply, opts...)
}

func (cc *ClientConn) invoke(
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

	replyBody, err := cc.mp.CallUnaryMethod(ctx,
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

func (cc *ClientConn) asInvoker(
	ctx context.Context,
	method string,
	req, reply interface{},
	_ *grpc.ClientConn,
	opts ...grpc.CallOption,
) error {
	return cc.invoke(ctx, method, req, reply, opts...)
}

// NewStream begins a streaming RPC.
func (cc *ClientConn) NewStream(
	ctx context.Context,
	desc *grpc.StreamDesc,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if cc.streamInterceptor != nil {
		return cc.streamInterceptor(ctx, desc, nil, method, cc.asStreamer, opts...)
	}
	return cc.newStream(ctx, desc, method, opts...)
}

func (cc *ClientConn) newStream(
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

	stream, err := cc.mp.NewStream(ctx, &header)
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (cc *ClientConn) asStreamer(
	ctx context.Context,
	desc *grpc.StreamDesc,
	_ *grpc.ClientConn,
	method string,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	return cc.newStream(ctx, desc, method, opts...)
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
func chainedUnaryInterceptors(cc *ClientConn) grpc.UnaryClientInterceptor {
	interceptors := cc.chainUnaryInterceptors
	if cc.unaryInterceptor != nil {
		interceptors = append(
			[]grpc.UnaryClientInterceptor{cc.unaryInterceptor},
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
func chainedStreamInterceptors(cc *ClientConn) grpc.StreamClientInterceptor {
	interceptors := cc.chainStreamInterceptors
	if cc.streamInterceptor != nil {
		interceptors = append(
			[]grpc.StreamClientInterceptor{cc.streamInterceptor},
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
