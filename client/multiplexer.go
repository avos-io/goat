package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"

	"github.com/avos-io/goat"
	wrapped "github.com/avos-io/goat/gen"
	spb "google.golang.org/genproto/googleapis/rpc/status"
)

type RpcMultiplexer struct {
	rw       goat.RpcReadWriter
	handlers map[string]chan *wrapped.Rpc

	ctx    context.Context
	cancel context.CancelFunc

	mutex sync.Mutex
	rErr  error
	codec encoding.Codec
}

func NewRpcMultiplexer(rw goat.RpcReadWriter) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		rw:       rw,
		handlers: make(map[string]chan *wrapped.Rpc),
		codec:    encoding.GetCodec(proto.Name),
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

	log.Info().Msg("RpcMultiplexer: Close")
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
	header *wrapped.RequestHeader,
	body *wrapped.Body,
) (*wrapped.Body, error) {

	if err := rm.readErrorIfDone(); err != nil {
		return nil, err
	}

	streamId := uuid.NewString()

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
) (string, goat.RpcReadWriter, func(), error) {

	if err := rm.readErrorIfDone(); err != nil {
		return "", nil, nil, err
	}

	streamId := uuid.NewString()

	respChan := make(chan *wrapped.Rpc, 1)
	rm.registerHandler(streamId, respChan)

	teardown := func() {
		rm.unregisterHandler(streamId)
	}

	rw := goat.NewFnReadWriter(
		func(ctx context.Context) (*wrapped.Rpc, error) {
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
		func(ctx context.Context, rpc *wrapped.Rpc) error {
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
			log.Error().Err(err).Msg("Mux: readLoop error")
			return err
		}

		rm.handleResponse(rpc)
	}
}

func (rm *RpcMultiplexer) handleResponse(rpc *wrapped.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	ch, ok := rm.handlers[rpc.GetId()]
	if !ok {
		log.Error().Msgf("Mux: unhandled Rpc %s", rpc.GetId())
		return
	}
	ch <- rpc
}

func (rm *RpcMultiplexer) registerHandler(id string, c chan *wrapped.Rpc) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rm.handlers[id] = c
}

func (rm *RpcMultiplexer) unregisterHandler(id string) {
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
