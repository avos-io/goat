package testutil

import (
	"context"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
)

type TestConn struct {
	ReadChan  chan ReadReturn
	WriteChan chan *goatorepo.Rpc
}

type ReadReturn struct {
	Rpc *goatorepo.Rpc
	Err error
}

func NewTestConn() *TestConn {
	conn := TestConn{
		ReadChan:  make(chan ReadReturn),
		WriteChan: make(chan *goatorepo.Rpc),
	}
	return &conn
}

func (c *TestConn) Read(ctx context.Context) (*goatorepo.Rpc, error) {
	select {
	case rr := <-c.ReadChan:
		return rr.Rpc, rr.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *TestConn) Write(ctx context.Context, rpc *goatorepo.Rpc) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.WriteChan <- rpc:
		return nil
	}
}
