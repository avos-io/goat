package testutil

import (
	"context"
	"sync"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
)

type TestConn struct {
	ReadChan  chan ReadReturn
	WriteChan chan *goatorepo.Rpc
	ErrorChan chan error

	error   error
	readers int
	writers int
	lock    sync.Mutex
}

type ReadReturn struct {
	Rpc *goatorepo.Rpc
	Err error
}

func NewTestConn() *TestConn {
	conn := TestConn{
		ReadChan:  make(chan ReadReturn),
		WriteChan: make(chan *goatorepo.Rpc),
		ErrorChan: make(chan error, 2),
	}
	return &conn
}

func (c *TestConn) Read(ctx context.Context) (*goatorepo.Rpc, error) {
	c.lock.Lock()
	c.readers++
	err := c.error
	c.lock.Unlock()
	defer func() { c.lock.Lock(); c.readers--; c.lock.Unlock() }()

	if err != nil {
		return nil, err
	}

	select {
	case rr := <-c.ReadChan:
		return rr.Rpc, rr.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-c.ErrorChan:
		return nil, err
	}
}

func (c *TestConn) Write(ctx context.Context, rpc *goatorepo.Rpc) error {
	c.lock.Lock()
	c.writers++
	err := c.error
	c.lock.Unlock()
	defer func() { c.lock.Lock(); c.writers--; c.lock.Unlock() }()

	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.WriteChan <- rpc:
		return nil
	case err := <-c.ErrorChan:
		return err
	}
}

func (c *TestConn) CurrentReaders() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.readers
}

func (c *TestConn) CurrentWriters() int {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.writers
}

func (c *TestConn) InjectError(err error) {
	c.lock.Lock()
	c.error = err
	c.lock.Unlock()

	c.ErrorChan <- err
	c.ErrorChan <- err
}
