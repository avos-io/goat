package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/types"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

type RpcMultiplexer struct {
	rw       types.RpcReadWriter
	handlers map[uint64]chan *goatorepo.Rpc

	ctx    context.Context
	cancel context.CancelFunc

	mutex         sync.Mutex
	streamCounter uint64
	rErr          error

	codec encoding.CodecV2
}

func NewRpcMultiplexer(rw types.RpcReadWriter) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		rw:       rw,
		handlers: make(map[uint64]chan *goatorepo.Rpc),
		codec:    encoding.GetCodecV2(proto.Name),
	}

	rm.ctx, rm.cancel = context.WithCancel(context.Background())

	go func() {
		err := rm.readLoop()
		rm.closeError(err)
	}()

	return rm
}

func (rm *RpcMultiplexer) Close() {
	rm.closeError(nil)
}

func (rm *RpcMultiplexer) closeError(err error) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	log.Trace().Msg("RpcMultiplexer: Close")
	rm.cancel()

	if err != nil {
		rm.rErr = err
		for id, ch := range rm.handlers {
			close(ch)
			delete(rm.handlers, id)
		}
	}
}

func (rm *RpcMultiplexer) CallUnaryMethod(
	ctx context.Context,
	header *goatorepo.RequestHeader,
	body *goatorepo.Body,
	statsHandlers []stats.Handler,
) (*goatorepo.Body, error) {

	if err := rm.readErrorIfDone(); err != nil {
		return nil, err
	}

	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *goatorepo.Rpc, 1)

	rm.registerHandler(streamId, respChan)
	defer rm.unregisterHandler(streamId)

	rpc := goatorepo.Rpc{
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
	case resp, ok := <-respChan:
		if !ok {
			return nil, fmt.Errorf("respChan closed")
		}
		for _, sh := range statsHandlers {
			headers, _ := internal.ToMetadata(resp.GetHeader().Headers)

			sh.HandleRPC(ctx, &stats.InHeader{
				Client:     true,
				FullMethod: header.Method,
				Header:     headers,
			})
		}
		if resp.Status != nil {
			return nil, status.FromProto(&spb.Status{
				Code:    resp.Status.Code,
				Message: resp.Status.Message,
				Details: resp.Status.Details,
			}).Err()
		}
		if resp.Body != nil {
			return resp.Body, nil
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
) (uint64, types.RpcReadWriter, func(), error) {

	if err := rm.readErrorIfDone(); err != nil {
		return 0, nil, nil, err
	}

	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	respChan := make(chan *goatorepo.Rpc, 1)
	rm.registerHandler(streamId, respChan)

	teardown := func() {
		rm.unregisterHandler(streamId)
	}

	rw := internal.NewFnReadWriter(
		func(ctx context.Context) (*goatorepo.Rpc, error) {
			select {
			case rpc, ok := <-respChan:
				if !ok {
					if err := rm.readErrorIfDone(); err != nil {
						return nil, err
					}
					return nil, fmt.Errorf("respChan closed")
				}
				return rpc, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
		func(ctx context.Context, rpc *goatorepo.Rpc) error {
			err := rm.rw.Write(ctx, rpc)
			if err != nil {
				if rErr := rm.readErrorIfDone(); rErr != nil {
					return rErr
				}
			}
			return err
		},
	)
	return streamId, rw, teardown, nil
}

func (rm *RpcMultiplexer) readLoop() error {
	for {
		rpc, err := rm.rw.Read(rm.ctx)

		if err != nil {
			log.Trace().Err(err).Msg("Mux: readLoop error")
			return err
		}

		rm.handleResponse(rpc)
	}
}

func (rm *RpcMultiplexer) handleResponse(rpc *goatorepo.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	ch, ok := rm.handlers[rpc.GetId()]
	if !ok {
		// TODO: getting log lines from here after cancelling streams
		log.Error().Msgf("Mux: unhandled Rpc %d", rpc.GetId())
		return
	}
	ch <- rpc
}

func (rm *RpcMultiplexer) registerHandler(id uint64, c chan *goatorepo.Rpc) {
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

func (rm *RpcMultiplexer) readErrorIfDone() error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	return rm.rErr
}
