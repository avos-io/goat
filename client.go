package goat

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/internal/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

type ClientConn struct {
	mp *client.RpcMultiplexer

	codec encoding.Codec

	unaryInterceptor  grpc.UnaryClientInterceptor
	streamInterceptor grpc.StreamClientInterceptor

	sourceAddress, destAddress string

	statsHandlers []stats.Handler
}

var _ grpc.ClientConnInterface = (*ClientConn)(nil)

func NewClientConn(conn RpcReadWriter, source, dest string, opts ...DialOption) *ClientConn {
	cc := ClientConn{
		mp:            client.NewRpcMultiplexer(conn),
		codec:         encoding.GetCodec(proto.Name),
		sourceAddress: source,
		destAddress:   dest,
	}

	for _, opt := range opts {
		opt.apply(&cc)
	}

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
	if len(opts) > 0 {
		log.Panic().Msg("Invoke opts unsupported")
	}

	var err error
	var statsBegin *stats.Begin
	for _, sh := range cc.statsHandlers {
		ctx = sh.TagRPC(ctx, &stats.RPCTagInfo{
			FullMethodName: method,
		})

		beginTime := time.Now()
		statsBegin = &stats.Begin{
			BeginTime:      beginTime,
			IsClientStream: false,
			IsServerStream: false,
		}
		sh.HandleRPC(ctx, statsBegin)
	}
	defer func() {
		for _, sh := range cc.statsHandlers {
			end := &stats.End{
				BeginTime: statsBegin.BeginTime,
				EndTime:   time.Now(),
			}
			if err != nil && err != io.EOF {
				end.Error = err
			}
			sh.HandleRPC(ctx, end)
		}
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
			FullMethod: method,
			Header:     mdHeaders,
		})
		sh.HandleRPC(ctx, &stats.OutPayload{
			Payload:  args,
			Data:     body,
			Length:   len(body),
			SentTime: time.Now(),
		})
		// Unary trailers? Do we support that?
		//sh.HandleRPC(ctx, &stats.OutTrailer{
		//	Trailer: sts.GetTrailers(),
		//})
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
			Data: body,
		},
		cc.statsHandlers,
	)

	if err != nil {
		log.Error().Err(err).Msg("Invoke CallUnaryMethod")
		return err
	}

	err = cc.codec.Unmarshal(replyBody.GetData(), reply)
	if err != nil {
		log.Error().Err(err).Msg("Invoke Unmarshal")
	}

	for _, sh := range cc.statsHandlers {
		sh.HandleRPC(ctx, &stats.InPayload{
			Client:   true,
			Payload:  reply,
			Data:     replyBody.GetData(),
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
	if len(opts) > 0 {
		log.Panic().Msg("NewStream: opts unsupported")
	}

	id, rw, teardown, err := cc.mp.NewStreamReadWriter(ctx)
	if err != nil {
		return nil, err
	}

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

	return client.NewStream(
		ctx,
		id,
		method,
		rw,
		teardown,
		cc.sourceAddress,
		cc.destAddress,
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
