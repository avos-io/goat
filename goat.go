package goat

import (
	"context"
	"fmt"

	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"nhooyr.io/websocket"

	rpcproto "github.com/avos-io/goat/gen"
)

type RpcReadWriter interface {
	Read(context.Context) (*rpcproto.Rpc, error)
	Write(context.Context, *rpcproto.Rpc) error
}

func NewFnReadWriter(
	r func(context.Context) (*rpcproto.Rpc, error),
	w func(context.Context, *rpcproto.Rpc) error,
) RpcReadWriter {
	return &fnReadWriter{r, w}
}

type fnReadWriter struct {
	r func(context.Context) (*rpcproto.Rpc, error)
	w func(context.Context, *rpcproto.Rpc) error
}

func (frw *fnReadWriter) Read(ctx context.Context) (*rpcproto.Rpc, error) {
	return frw.r(ctx)
}

func (frw *fnReadWriter) Write(ctx context.Context, rpc *rpcproto.Rpc) error {
	return frw.w(ctx, rpc)
}

// NOTE: this wouldn't actually live in the same package, but it's here for now
type WebsocketRpcReadWriter struct {
	conn  *websocket.Conn
	codec encoding.Codec
}

func NewWebsocketRpcReadWriter(conn *websocket.Conn) RpcReadWriter {
	return &WebsocketRpcReadWriter{
		conn:  conn,
		codec: encoding.GetCodec(proto.Name),
	}
}

func (wrw *WebsocketRpcReadWriter) Read(ctx context.Context) (*rpcproto.Rpc, error) {
	msgType, data, err := wrw.conn.Read(ctx)

	if err != nil {
		return nil, err
	}

	if msgType != websocket.MessageBinary {
		return nil, fmt.Errorf("WebsocketRpcReadWriter: invalid recv type %d", msgType)
	}

	var rpc rpcproto.Rpc
	err = wrw.codec.Unmarshal(data, &rpc)
	if err != nil {
		return nil, err
	}

	return &rpc, nil
}

func (wrw *WebsocketRpcReadWriter) Write(ctx context.Context, rpc *rpcproto.Rpc) error {
	bytes, err := wrw.codec.Marshal(rpc)
	if err != nil {
		return err
	}
	return wrw.conn.Write(ctx, websocket.MessageBinary, bytes)
}
