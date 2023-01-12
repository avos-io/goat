package goat

import (
	"context"
	"fmt"

	"github.com/avos-io/goat/internal"
)

// NewGoatOverChannel turns an input and an output channel into an RpcReadWriter.
func NewGoatOverChannel(inQ chan *Rpc, outQ chan *Rpc) RpcReadWriter {
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
