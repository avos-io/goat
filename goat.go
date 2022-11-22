// Package GOAT (gRPC Over Any Transport) is a gRPC client and server
// implementation which runs over any transport which implements RpcReadWriter.
//
// The idea is that gRPC requests and responses are serialised into a wrapper
// protobuf (wrapped.Rpc).
package goat

import (
	"context"

	wrapped "github.com/avos-io/goat/gen"
)

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls and their metadata.
type RpcReadWriter interface {
	Read(context.Context) (*wrapped.Rpc, error)
	Write(context.Context, *wrapped.Rpc) error
}
