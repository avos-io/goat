package testutil

import (
	"context"

	wrapped "github.com/avos-io/goat/gen"
)

type TestConn struct {
	ReadChan  chan ReadReturn
	WriteChan chan *wrapped.Rpc
}

type ReadReturn struct {
	Rpc *wrapped.Rpc
	Err error
}

func NewTestConn() *TestConn {
	conn := TestConn{
		ReadChan:  make(chan ReadReturn),
		WriteChan: make(chan *wrapped.Rpc),
	}
	return &conn
}

func (c *TestConn) Read(ctx context.Context) (*wrapped.Rpc, error) {
	select {
	case rr := <-c.ReadChan:
		return rr.Rpc, rr.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *TestConn) Write(ctx context.Context, rpc *wrapped.Rpc) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.WriteChan <- rpc:
		return nil
	}
}
