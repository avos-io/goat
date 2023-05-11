package goat

import (
	"context"
	"sync"
)

type demuxConn struct {
	r chan *Rpc
	w chan *Rpc
}

// Wraps a Goat Server, demultiplexing IO.
type Demux struct {
	ctx    context.Context
	cancel context.CancelFunc

	rw RpcReadWriter

	conns struct {
		sync.Mutex
		value map[string]*demuxConn
	}

	demuxOn         func(*Rpc) string
	onNewConnection func(RpcReadWriter)
}

// NewDemux returns a new Demux which demultiplexex RPCs from the given |rw| 
// into |onNewConnection| based the identity returned from |demuxOn|.
func NewDemux(
	ctx context.Context,
	rw RpcReadWriter,
	demuxOn func(*Rpc) string,
	onNewConnection func(RpcReadWriter),
) *Demux {
	ctx, cancel := context.WithCancel(ctx)

	gsd := &Demux{
		ctx:             ctx,
		cancel:          cancel,
		rw:              rw,
		demuxOn:         demuxOn,
		onNewConnection: onNewConnection,
	}
	gsd.conns.value = make(map[string]*demuxConn)

	return gsd
}

func (gsd *Demux) Stop() {
	gsd.cancel()
}

func (gsd *Demux) Run() {
	for {
		rpc, err := gsd.rw.Read(gsd.ctx)
		if err != nil {
			return
		}
		gsd.conns.Lock()
		id := gsd.demuxOn(rpc)
		conn, ok := gsd.conns.value[id]
		if !ok {
			conn = gsd.newConnLocked(id)
		}
		gsd.conns.Unlock()

		conn.r <- rpc
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
				err := gsd.rw.Write(gsd.ctx, rpc)
				if err != nil {
					return
				}
			}
		}
	}()

	gsd.conns.value[id] = c

	go gsd.onNewConnection(NewGoatOverChannel(c.r, c.w))

	return c
}
