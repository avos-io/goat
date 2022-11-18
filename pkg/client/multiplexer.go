package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/avos-io/goat"
	rpcheader "github.com/avos-io/goat/gen"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"
)

type RpcMultiplexer struct {
	rw       goat.RpcReadWriter
	handlers map[uint64]chan *rpcheader.Rpc

	ctx    context.Context
	cancel context.CancelFunc

	streamCounter uint64
	mutex         sync.Mutex

	codec encoding.Codec
}

func NewRpcMultiplexer(rw goat.RpcReadWriter) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		rw:       rw,
		handlers: make(map[uint64]chan *rpcheader.Rpc),
		codec:    encoding.GetCodec(proto.Name),
	}

	rm.ctx, rm.cancel = context.WithCancel(context.Background())
	go rm.readLoop()

	return rm
}

func (rm *RpcMultiplexer) Close() {
	rm.cancel()
}

func (rm *RpcMultiplexer) CallUnaryMethod(
	ctx context.Context,
	header *rpcheader.RequestHeader,
	body *rpcheader.Body,
) (*rpcheader.Body, error) {
	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *rpcheader.Rpc, 1)

	rm.registerHandler(streamId, respChan)
	defer rm.unregisterHandler(streamId)

	rpc := rpcheader.Rpc{
		Id:     streamId,
		Header: header,
		Body:   body,
	}

	err := rm.rw.Write(ctx, &rpc)
	if err != nil {
		log.Printf("CallUnaryMethod: conn.Write %v", err)
		return nil, err
	}

	select {
	case resp := <-respChan:
		if resp.Body != nil {
			return resp.Body, nil
		}
		if resp.Status != nil {
			return nil, status.FromProto(&spb.Status{
				Code:    resp.Status.Code,
				Message: resp.Status.Message,
				Details: resp.Status.Details,
			}).Err()
		}
		return nil, fmt.Errorf("malformed response: no body or status")

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type streamReadWriter struct {
	rFn func(context.Context) (*rpcheader.Rpc, error)
	wFn func(context.Context, *rpcheader.Rpc) error
}

func (srw *streamReadWriter) Read(ctx context.Context) (*rpcheader.Rpc, error) {
	return srw.rFn(ctx)
}

func (srw *streamReadWriter) Write(ctx context.Context, rpc *rpcheader.Rpc) error {
	return srw.wFn(ctx, rpc)
}

// NewStreamReadWriter returns a new goat.RpcReadWriter which will read and
// write Rpcs using the returned id. Also returns a teardown function which will
// close the stream.
func (rm *RpcMultiplexer) NewStreamReadWriter(
	ctx context.Context,
) (uint64, goat.RpcReadWriter, func()) {
	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *rpcheader.Rpc, 1)
	rm.registerHandler(streamId, respChan)

	teardown := func() {
		rm.unregisterHandler(streamId)
	}

	rw := &streamReadWriter{
		rFn: func(ctx context.Context) (*rpcheader.Rpc, error) {
			select {
			case rpc, ok := <-respChan:
				if !ok {
					return nil, fmt.Errorf("respChan closed")
				}
				return rpc, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		wFn: rm.rw.Write,
	}
	return streamId, rw, teardown
}

func (rm *RpcMultiplexer) readLoop() {
	for {
		rpc, err := rm.rw.Read(rm.ctx)
		if err != nil {
			return
		}
		rm.handleResponse(rpc)
	}
}

func (rm *RpcMultiplexer) handleResponse(rpc *rpcheader.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if ch, ok := rm.handlers[rpc.Id]; ok {
		ch <- rpc
	}
}

func (rm *RpcMultiplexer) registerHandler(id uint64, c chan *rpcheader.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.handlers[id] = c
}

func (rm *RpcMultiplexer) unregisterHandler(id uint64) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if ch, ok := rm.handlers[id]; ok {
		close(ch)
	}

	delete(rm.handlers, id)
}
