// Package GOAT (gRPC Over Any Transport) is a gRPC client and server
// implementation which runs over any transport which implements RpcReadWriter.
//
// The idea is that gRPC requests and responses are serialised into a wrapper
// protobuf (wrapped.Rpc).
package goat

import (
	"context"

	proto "github.com/avos-io/goat/gen"
)

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls and their metadata.
type RpcReadWriter interface {
	Read(context.Context) (*proto.Rpc, error)
	Write(context.Context, *proto.Rpc) error
}

// NewFnReadWriter is a convenience wrapper to turn read and write functions
// into an RpcReadWriter.
func NewFnReadWriter(
	r func(context.Context) (*proto.Rpc, error),
	w func(context.Context, *proto.Rpc) error,
) RpcReadWriter {
	return &fnReadWriter{r, w}
}

type fnReadWriter struct {
	r func(context.Context) (*proto.Rpc, error)
	w func(context.Context, *proto.Rpc) error
}

func (frw *fnReadWriter) Read(ctx context.Context) (*proto.Rpc, error) {
	return frw.r(ctx)
}

func (frw *fnReadWriter) Write(ctx context.Context, rpc *proto.Rpc) error {
	return frw.w(ctx, rpc)
}
