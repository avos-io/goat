package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"

	wrapped "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/types"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

type RpcMultiplexer struct {
	rw       types.RpcReadWriter
	handlers map[uint64]chan *wrapped.Rpc

	ctx    context.Context
	cancel context.CancelFunc

	streamCounter uint64
	mutex         sync.Mutex

	codec encoding.Codec
}

func NewRpcMultiplexer(rw types.RpcReadWriter) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		rw:       rw,
		handlers: make(map[uint64]chan *wrapped.Rpc),
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
	header *wrapped.RequestHeader,
	body *wrapped.Body,
) (*wrapped.Body, error) {
	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *wrapped.Rpc, 1)

	rm.registerHandler(streamId, respChan)
	defer rm.unregisterHandler(streamId)

	rpc := wrapped.Rpc{
		Id:     streamId,
		Header: header,
		Body:   body,
	}

	err := rm.rw.Write(ctx, &rpc)
	if err != nil {
		log.Error().Err(err).Msg("CallUnaryMethod: conn.Write")
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

// NewStreamReadWriter returns a new goat.RpcReadWriter which will read and
// write Rpcs using the returned id. Also returns a teardown function which will
// close the stream.
func (rm *RpcMultiplexer) NewStreamReadWriter(
	ctx context.Context,
) (uint64, types.RpcReadWriter, func()) {
	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *wrapped.Rpc, 1)
	rm.registerHandler(streamId, respChan)

	teardown := func() {
		rm.unregisterHandler(streamId)
	}

	rw := internal.NewFnReadWriter(
		func(ctx context.Context) (*wrapped.Rpc, error) {
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
		rm.rw.Write,
	)
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

func (rm *RpcMultiplexer) handleResponse(rpc *wrapped.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	ch, ok := rm.handlers[rpc.GetId()]
	if !ok {
		log.Error().Msgf("Mux: unhandled Rpc %d", rpc.GetId())
		return
	}
	ch <- rpc
}

func (rm *RpcMultiplexer) registerHandler(id uint64, c chan *wrapped.Rpc) {
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
