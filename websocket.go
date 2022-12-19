package goat

import (
	"context"
	"errors"

	wrapped "github.com/avos-io/goat/gen"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

var errNonBinaryWebsocketMessage = errors.New("invalid websocket message: not binary")

type goatOverWebsocket struct {
	conn *websocket.Conn
}

func NewGoatOverWebsocket(ws *websocket.Conn) RpcReadWriter {
	return &goatOverWebsocket{ws}
}

func (ws *goatOverWebsocket) Read(ctx context.Context) (*wrapped.Rpc, error) {
	typ, data, err := ws.conn.Read(ctx)

	if err != nil {
		return nil, err
	}

	if typ != websocket.MessageBinary {
		return nil, errNonBinaryWebsocketMessage
	}

	var rpc wrapped.Rpc
	err = proto.Unmarshal(data, &rpc)
	if err != nil {
		return nil, err
	}

	return &rpc, nil
}

func (ws *goatOverWebsocket) Write(ctx context.Context, pkt *wrapped.Rpc) error {
	data, err := proto.Marshal(pkt)
	if err != nil {
		return err
	}

	return ws.conn.Write(ctx, websocket.MessageBinary, data)
}
