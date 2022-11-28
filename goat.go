// Package GOAT (gRPC Over Any Transport) is a gRPC client and server
// implementation which runs over any transport which implements RpcReadWriter.
//
// The idea is that gRPC requests and responses are serialised into a wrapper
// protobuf (wrapped.Rpc).
package goat

import (
	"context"
	"fmt"

	proto "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/internal"
	"google.golang.org/grpc/metadata"
)

type Rpc = proto.Rpc

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls and their metadata.
type RpcReadWriter interface {
	Read(context.Context) (*Rpc, error)
	Write(context.Context, *Rpc) error
}

// NewFnReadWriter is a convenience wrapper to turn read and write functions
// into an RpcReadWriter.
func NewFnReadWriter(
	r func(context.Context) (*Rpc, error),
	w func(context.Context, *Rpc) error,
) RpcReadWriter {
	return &fnReadWriter{r, w}
}

type fnReadWriter struct {
	r func(context.Context) (*Rpc, error)
	w func(context.Context, *Rpc) error
}

func (frw *fnReadWriter) Read(ctx context.Context) (*Rpc, error) {
	return frw.r(ctx)
}

func (frw *fnReadWriter) Write(ctx context.Context, rpc *Rpc) error {
	return frw.w(ctx, rpc)
}

// NewChannelReadWriter is a convenience wrapper to turn a read channel and a
// write channel into an RpcReadWriter
func NewChannelReadWriter(inQ chan *Rpc, outQ chan *Rpc) RpcReadWriter {
	return NewFnReadWriter(
		func(ctx context.Context) (*Rpc, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case rpc, ok := <-inQ:
				if !ok {
					return nil, fmt.Errorf("read channel closed")
				}
				return rpc, nil
			}
		},
		func(ctx context.Context, rpc *Rpc) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case outQ <- rpc:
				return nil
			}
		},
	)
}

// NewOutgoingContextWithHeaders creates a new outgoing context with the Rpc's
// headers attached as metadata.
func NewOutgoingContextWithHeaders(rpc *Rpc) (context.Context, error) {
	headers := rpc.GetHeader().GetHeaders()
	if headers == nil {
		return nil, fmt.Errorf("NewContextFromHeaders: headers nil")
	}

	md, err := internal.ToMetadata(headers)
	if err != nil {
		return nil, err
	}

	return metadata.NewOutgoingContext(context.Background(), md), nil
}
