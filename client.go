package goat

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/internal/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

type ClientConn struct {
	mp *client.RpcMultiplexer

	codec encoding.CodecV2

	unaryInterceptor  grpc.UnaryClientInterceptor
	streamInterceptor grpc.StreamClientInterceptor

	sourceAddress, destAddress string

	statsHandlers []stats.Handler
}

var _ grpc.ClientConnInterface = (*ClientConn)(nil)

func NewClientConn(conn RpcReadWriter, source, dest string, opts ...DialOption) *ClientConn {
	cc := ClientConn{
		mp:            client.NewRpcMultiplexer(conn),
		codec:         encoding.GetCodecV2(proto.Name),
		sourceAddress: source,
		destAddress:   dest,
	}

	for _, opt := range opts {
		opt.apply(&cc)
	}

	for _, sh := range cc.statsHandlers {
		sh.TagConn(context.Background(), &stats.ConnTagInfo{})
		sh.HandleConn(context.Background(), &stats.ConnBegin{
			Client: true,
		})
	}

	return &cc
}

func (cc *ClientConn) Close() {
	for _, sh := range cc.statsHandlers {
		sh.HandleConn(context.Background(), &stats.ConnEnd{
			Client: true,
		})
	}
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
		// NOTE: grpc.ClientConn is a concrete type which leaks out of package grpc;
		// since we're not that concrete type, we're forced to pass nil here.
		return cc.unaryInterceptor(ctx, method, args, reply, nil, cc.asInvoker, opts...)
	}
	return cc.invoke(ctx, method, args, reply, opts...)
}

func (cc *ClientConn) invoke(
	ctx context.Context,
	method string,
	args interface{},
	reply interface{},
	opts ...grpc.CallOption,
) error {
	if len(opts) > 1 {
		log.Panic().Msg("Invoke opts unsupported")
	} else if len(opts) == 1 {
		if _, ok := opts[0].(grpc.EmptyCallOption); !ok {
			log.Panic().Msg("Invoke opts unsupported")
		}
	}

	var err error

	beginTime := time.Now()
	ctx = internal.StatsStartServerRPC(cc.statsHandlers, true, beginTime, method, false, false, ctx)
	defer func() {
		internal.StatsEndRPC(cc.statsHandlers, true, beginTime, err, ctx)
	}()

	headers := headersFromContext(ctx)

	body, err := cc.codec.Marshal(args)
	if err != nil {
		log.Error().Err(err).Msg("Invoke Marshal")
		return err
	}

	for _, sh := range cc.statsHandlers {
		mdHeaders, _ := internal.ToMetadata(headers)

		sh.HandleRPC(ctx, &stats.OutHeader{
			Client:     true,
			FullMethod: method,
			Header:     mdHeaders,
		})

		sh.HandleRPC(ctx, &stats.OutPayload{
			Client:   true,
			Payload:  args,
			Length:   len(body),
			SentTime: time.Now(),
		})
	}

	var replyBody *goatorepo.Body
	replyBody, err = cc.mp.CallUnaryMethod(
		ctx,
		&goatorepo.RequestHeader{
			Method:      method,
			Headers:     headers,
			Source:      cc.sourceAddress,
			Destination: cc.destAddress,
		},
		&goatorepo.Body{
			Data: body.Materialize(),
		},
		cc.statsHandlers,
	)

	if err != nil {
		log.Error().Err(err).Msg("Invoke CallUnaryMethod")
		return err
	}

	buf := mem.NewBuffer(&replyBody.Data, nil)
	bs := mem.BufferSlice{buf}

	err = cc.codec.Unmarshal(bs, reply)
	if err != nil {
		log.Error().Err(err).Msg("Invoke Unmarshal")
	}

	for _, sh := range cc.statsHandlers {
		sh.HandleRPC(ctx, &stats.InPayload{
			Client:   true,
			Payload:  reply,
			Length:   len(replyBody.GetData()),
			RecvTime: time.Now(),
		})
	}

	return err
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
		// NOTE: grpc.ClientConn is a concrete type which leaks out of package grpc;
		// since we're not that concrete type, we're forced to pass nil here.
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
	if len(opts) > 1 {
		log.Panic().Msg("NewStream: opts unsupported")
	} else if len(opts) == 1 {
		if _, ok := opts[0].(grpc.EmptyCallOption); !ok {
			log.Panic().Msg("NewStream: opts unsupported")
		}
	}

	id, rw, teardown, err := cc.mp.NewStreamReadWriter(ctx)
	if err != nil {
		return nil, err
	}

	beginTime := time.Now()
	for _, sh := range cc.statsHandlers {
		ctx = sh.TagRPC(ctx, &stats.RPCTagInfo{
			FullMethodName: method,
		})
		sh.HandleRPC(ctx, &stats.Begin{
			Client:         true,
			BeginTime:      beginTime,
			IsClientStream: desc.ClientStreams,
			IsServerStream: desc.ServerStreams,
		})
	}
	defer func() {
		if err != nil {
			now := time.Now()
			for _, sh := range cc.statsHandlers {
				sh.HandleRPC(ctx, &stats.End{
					Client:    true,
					BeginTime: beginTime,
					EndTime:   now,
					Error:     err,
				})
			}
		}
	}()

	// open stream
	rpc := goatorepo.Rpc{
		Id: id,
		Header: &goatorepo.RequestHeader{
			Method:      method,
			Headers:     headersFromContext(ctx),
			Source:      cc.sourceAddress,
			Destination: cc.destAddress,
		},
	}
	err = rw.Write(ctx, &rpc)
	if err != nil {
		log.Error().Err(err).Msg("NewStream: failed to open")
		return nil, err
	}

	for _, sh := range cc.statsHandlers {
		md, _ := internal.ToMetadata(rpc.Header.Headers)
		sh.HandleRPC(ctx, &stats.OutHeader{
			Client:     true,
			FullMethod: method,
			Header:     md,
		})
	}

	return client.NewStream(
		ctx,
		id,
		method,
		rw,
		teardown,
		cc.sourceAddress,
		cc.destAddress,
		cc.statsHandlers,
		beginTime,
	), nil
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

// headersFromContext transforms metadata from the given context into KeyValues
// which can be used as part of the wrapped header.
func headersFromContext(ctx context.Context) []*goatorepo.KeyValue {
	h := []*goatorepo.KeyValue{}
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		h = append(h, internal.ToKeyValue(md)...)
	}
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		ms := int64(timeout / time.Millisecond)
		if ms <= 0 {
			ms = 1
		}
		h = append(h, &goatorepo.KeyValue{
			Key:   "GRPC-Timeout",
			Value: fmt.Sprintf("%dm", ms),
		})
	}
	return h
}
