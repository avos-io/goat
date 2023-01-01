package goat

import (
	"context"
	"fmt"

	"github.com/avos-io/goat/internal"
)

type ReadReturn struct {
	Rpc *Rpc
	Err error
}

// NewGoatOverChannel is a convenience wrapper to turn a read channel and a
// write channel into an RpcReadWriter
func NewGoatOverChannel(inQ chan ReadReturn, outQ chan *Rpc) RpcReadWriter {
	read := func(ctx context.Context) (*Rpc, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case ret, ok := <-inQ:
			if !ok {
				return nil, fmt.Errorf("read channel closed")
			}
			if ret.Err != nil {
				return nil, ret.Err
			}
			return ret.Rpc, nil
		}
	}

	write := func(ctx context.Context, rpc *Rpc) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case outQ <- rpc:
			return nil
		}
	}

	return internal.NewFnReadWriter(read, write)
}

// TODO(mjb):
// Can't decide whether I need errors explicitly in the channel item, or if
// closing the channel on errors will be sufficient...
func NewGoatOverChannelNoErrors(inQ chan *Rpc, outQ chan *Rpc) RpcReadWriter {
	read := func(ctx context.Context) (*Rpc, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case rpc, ok := <-inQ:
			if !ok {
				return nil, fmt.Errorf("read channel closed")
			}
			return rpc, nil
		}
	}

	write := func(ctx context.Context, rpc *Rpc) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case outQ <- rpc:
			return nil
		}
	}

	return internal.NewFnReadWriter(read, write)
}
