package goat

import (
	"context"

	wrapped "github.com/avos-io/goat/gen"
)

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls.
type RpcReadWriter interface {
	Read(context.Context) (*wrapped.Rpc, error)
	Write(context.Context, *wrapped.Rpc) error
}

// NewFnReadWriter is a convenience wrapper to turn read and write functions
// into an RpcReadWriter.
func NewFnReadWriter(
	r func(context.Context) (*wrapped.Rpc, error),
	w func(context.Context, *wrapped.Rpc) error,
) RpcReadWriter {
	return &fnReadWriter{r, w}
}

type fnReadWriter struct {
	r func(context.Context) (*wrapped.Rpc, error)
	w func(context.Context, *wrapped.Rpc) error
}

func (frw *fnReadWriter) Read(ctx context.Context) (*wrapped.Rpc, error) {
	return frw.r(ctx)
}

func (frw *fnReadWriter) Write(ctx context.Context, rpc *wrapped.Rpc) error {
	return frw.w(ctx, rpc)
}
