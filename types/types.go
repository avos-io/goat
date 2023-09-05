package types

import (
	"context"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
)

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls and their metadata.
type RpcReadWriter interface {
	Read(context.Context) (*goatorepo.Rpc, error)
	Write(context.Context, *goatorepo.Rpc) error
}
