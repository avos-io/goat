package goat

import (
	"context"
	"sync"
)

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

	conns struct {
		sync.Mutex
		value map[string]*demuxConn
	}
}

func NewDemux(
	server *Server,
	r chan *Rpc,
	w chan *Rpc,
) *Demux {
	ctx, cancel := context.WithCancel(context.Background())

	gsd := &Demux{
		ctx:    ctx,
		cancel: cancel,
		server: server,
		r:      r,
		w:      w,
	}
	gsd.conns.value = make(map[string]*demuxConn)

	return gsd
}

func (gsd *Demux) Stop() {
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
			conn, ok := gsd.conns.value[rpc.Header.Source]
			if !ok {
				conn = gsd.newConnLocked(rpc.Header.Source)
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
