package client

import (
	"context"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/avos-io/grpc-websockets/internal/multiplexer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"nhooyr.io/websocket"
)

/*
type ClientConnInterface interface {
	// Invoke performs a unary RPC and returns after the response is received
	// into reply.
	Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...CallOption) error
	// NewStream begins a streaming RPC.
	NewStream(ctx context.Context, desc *StreamDesc, method string, opts ...CallOption) (ClientStream, error)
}
*/

type WebsocketClientConn struct {
	mp *multiplexer.RpcMultiplexer
}

func NewWebsocketClientConn(conn *websocket.Conn) grpc.ClientConnInterface {
	return &WebsocketClientConn{
		mp: multiplexer.NewRpcMultiplexer(conn),
	}
}

// ClientConnInterface
func (wcc *WebsocketClientConn) Invoke(ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error {
	// TODO: what to do with `opts` ?

	codec := encoding.GetCodec(proto.Name)
	body, err := codec.Marshal(args)
	if err != nil {
		return err
	}

	replyBody, err := wcc.mp.CallUnaryMethod(ctx,
		&rpcheader.RequestHeader{
			Method: method,
		},
		&rpcheader.Body{
			Data: body,
		})

	if err != nil {
		return err
	}

	return codec.Unmarshal(replyBody.Data, reply)
}

// ClientConnInterface
func (*WebsocketClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}
