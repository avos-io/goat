// Package GOAT (gRPC Over Any Transport) is a gRPC client and server
// implementation which runs over any transport which implements RpcReadWriter.
//
// The idea is that gRPC requests and responses are serialised into a wrapper
// protobuf (wrapped.Rpc).
package goat

import (
	proto "github.com/avos-io/goat/gen/protorepo/goat"
	"github.com/avos-io/goat/types"
)

// Rpc is the fundamental type in Goat: the generic protobuf structure into
// which all goat messages are serialised.
type Rpc = proto.Rpc

// RpcHeader represents the header of a Goat Rpc.
type RpcHeader = proto.RequestHeader

// RpcReadWriter is the generic interface used by Goat's client and servers.
// It utilises the wrapped.Rpc protobuf format for generically wrapping gRPC
// calls and their metadata.
type RpcReadWriter = types.RpcReadWriter
