package goat

import (
	"context"
	"sync"
)

type rpcToId func(*Rpc) string

type demuxConn struct {
	r chan *Rpc
	w chan *Rpc
}

// Wraps a Goat Server, demultiplexing IO based on source address.
type Demux struct {
	ctx    context.Context
	cancel context.CancelFunc

	server *Server
	r      chan *Rpc
	w      chan *Rpc
	io     RpcReadWriter

	conns struct {
		sync.Mutex
		value map[string]*demuxConn
	}

	cb rpcToId
}

func NewDemux(
	ctx context.Context,
	server *Server,
	r chan *Rpc,
	w chan *Rpc,
	cb rpcToId,
) *Demux {
	ctx, cancel := context.WithCancel(ctx)

	gsd := &Demux{
		ctx:    ctx,
		cancel: cancel,
		server: server,
		r:      r,
		w:      w,
		io:     NewGoatOverChannel(w, r),
		cb:     cb,
	}
	gsd.conns.value = make(map[string]*demuxConn)

	return gsd
}

func (gsd *Demux) Stop() {
	gsd.server.Stop()
	gsd.cancel()
}

func (gsd *Demux) Run() {
	for {
		select {
		case <-gsd.ctx.Done():
			return
		case rpc, ok := <-gsd.r:
			if !ok {
				return
			}
			gsd.conns.Lock()
			id := gsd.cb(rpc)
			conn, ok := gsd.conns.value[id]
			if !ok {
				conn = gsd.newConnLocked(id)
			}
			gsd.conns.Unlock()

			conn.r <- rpc
		}
	}
}

func (gsd *Demux) Cancel(id string) {
	gsd.conns.Lock()
	defer gsd.conns.Unlock()

	if conn, ok := gsd.conns.value[id]; ok {
		close(conn.r)
		close(conn.w)
	}

	delete(gsd.conns.value, id)
}

func (gsd *Demux) IO() RpcReadWriter {
	return gsd.io
}

func (gsd *Demux) newConnLocked(id string) *demuxConn {
	c := &demuxConn{
		r: make(chan *Rpc),
		w: make(chan *Rpc),
	}

	go func() {
		for {
			select {
			case <-gsd.ctx.Done():
				return
			case rpc, ok := <-c.w:
				if !ok {
					return
				}
				gsd.w <- rpc
			}
		}
	}()

	gsd.conns.value[id] = c

	go gsd.server.Serve(NewGoatOverChannel(c.r, c.w))

	return c
}
