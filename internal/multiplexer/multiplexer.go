package multiplexer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	spb "google.golang.org/genproto/googleapis/rpc/status"
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
	conn          SimpleWebsocketConn
	ctx           context.Context
	cancel        context.CancelFunc
	streamCounter uint64
	mutex         sync.Mutex
	handlers      map[uint64]chan *rpcheader.Rpc
}

func NewRpcMultiplexer(conn SimpleWebsocketConn) *RpcMultiplexer {
	rm := &RpcMultiplexer{
		conn:     conn,
		handlers: make(map[uint64]chan *rpcheader.Rpc),
	}

	rm.ctx, rm.cancel = context.WithCancel(context.Background())
	go rm.readLoop()

	return rm
}

func (rm *RpcMultiplexer) Close() {
	rm.cancel()
}

func (rm *RpcMultiplexer) CallUnaryMethod(ctx context.Context, header *rpcheader.RequestHeader, body *rpcheader.Body) (*rpcheader.Body, error) {
	codec := encoding.GetCodec(proto.Name)
	streamId := atomic.AddUint64(&rm.streamCounter, 1)

	rpcBody, err := codec.Marshal(&rpcheader.Rpc{
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

func (rm *RpcMultiplexer) readLoop() {
	for {
		msgType, data, err := rm.conn.Read(rm.ctx)

		if err != nil {
			return
		}

		if msgType != websocket.MessageBinary {
			return
		}

		codec := encoding.GetCodec(proto.Name)

		var rpc rpcheader.Rpc
		err = codec.Unmarshal(data, &rpc)
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
