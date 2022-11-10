package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"
	"nhooyr.io/websocket"
)

// Just the basics we need, to allow for testing
type SimpleWebsocketConn interface {
	Read(ctx context.Context) (websocket.MessageType, []byte, error)
	Write(ctx context.Context, typ websocket.MessageType, p []byte) error
}

type RpcMultiplexer struct {
	conn     SimpleWebsocketConn
	handlers map[uint64]chan *rpcheader.Rpc

	ctx    context.Context
	cancel context.CancelFunc

	streamCounter atomic.Uint64
	mutex         sync.Mutex

	codec encoding.Codec
}

func NewRpcMultiplexer(conn SimpleWebsocketConn) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		conn:     conn,
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
	streamId := rm.streamCounter.Add(1)

	rpcBody, err := rm.codec.Marshal(&rpcheader.Rpc{
		Id:     streamId,
		Header: header,
		Body:   body,
	})
	if err != nil {
		log.Printf("CallUnaryMethod: codec.Marshall %v", err)
		return nil, err
	}

	respChan := make(chan *rpcheader.Rpc, 1)

	rm.registerHandler(streamId, respChan)
	defer rm.unregisterHandler(streamId)

	err = rm.conn.Write(ctx, websocket.MessageBinary, rpcBody)
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

func (rm *RpcMultiplexer) NewStream(
	ctx context.Context,
	header *rpcheader.RequestHeader,
) (grpc.ClientStream, error) {
	streamId := rm.streamCounter.Add(1)

	respChan := make(chan *rpcheader.Rpc, 1)
	rm.registerHandler(streamId, respChan)

	teardown := func() {
		rm.unregisterHandler(streamId)
	}
	r := func(ctx context.Context) (*rpcheader.Rpc, error) {
		select {
		case rpc := <-respChan:
			// log.Printf("[client r]: %v", rpc)
			return rpc, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	w := func(ctx context.Context, rpc *rpcheader.Rpc) error {
		// log.Printf("[client w]: %v", rpc)
		bytes, err := rm.codec.Marshal(rpc)
		if err != nil {
			return err
		}
		return rm.conn.Write(ctx, websocket.MessageBinary, bytes)
	}

	log.Printf("NewStream: stream %d", streamId)
	rpc := rpcheader.Rpc{
		Id:     streamId,
		Header: header,
	}
	err := w(ctx, &rpc)
	if err != nil {
		log.Printf("NewStream: failed to open, %v", err)
		return nil, err
	}

	stream := newClientStream(ctx, rm, streamId, header.GetMethod(), r, w, teardown)
	go func() {
		if err := stream.readLoop(); err != nil {
			log.Printf("NewStream: stream %d ended with %v", streamId, err)
		}
	}()

	return stream, nil
}

func (rm *RpcMultiplexer) readLoop() {
	for {
		msgType, data, err := rm.conn.Read(rm.ctx)

		if err != nil {
			return
		}

		if msgType != websocket.MessageBinary {
			return
		}

		var rpc rpcheader.Rpc
		err = rm.codec.Unmarshal(data, &rpc)
		if err != nil {
			return
		}

		rm.handleResponse(&rpc)
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
