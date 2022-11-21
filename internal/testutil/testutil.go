package testutil

import (
	"context"

	rpcheader "github.com/avos-io/goat/gen"
)

type TestConn struct {
	ReadChan  chan ReadReturn
	WriteChan chan *rpcheader.Rpc
}

type ReadReturn struct {
	Rpc *rpcheader.Rpc
	Err error
}

func NewTestConn() *TestConn {
	conn := TestConn{
		ReadChan:  make(chan ReadReturn),
		WriteChan: make(chan *rpcheader.Rpc),
	}
	return &conn
}

func (c *TestConn) Read(ctx context.Context) (*rpcheader.Rpc, error) {
	select {
	case rr := <-c.ReadChan:
		return rr.Rpc, rr.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *TestConn) Write(ctx context.Context, rpc *rpcheader.Rpc) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.WriteChan <- rpc:
		return nil
	}
}
