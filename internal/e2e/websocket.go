package e2e

import (
	"context"

	"github.com/avos-io/goat"
	wrapped "github.com/avos-io/goat/gen"
	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"
)

type goatOverWebsocket struct {
	conn *websocket.Conn
}

func NewGoatOverWebsocket(ws *websocket.Conn) goat.RpcReadWriter {
	return &goatOverWebsocket{ws}
}

func (ws *goatOverWebsocket) Read(ctx context.Context) (*wrapped.Rpc, error) {
	typ, data, err := ws.conn.Read(ctx)

	if err != nil {
		return nil, err
	}

	if typ != websocket.MessageBinary {
		panic("unexpected type")
	}

	var rpc wrapped.Rpc
	err = proto.Unmarshal(data, &rpc)
	if err != nil {
		panic(err)
	}

	return &rpc, nil
}

func (ws *goatOverWebsocket) Write(ctx context.Context, pkt *wrapped.Rpc) error {
	data, err := proto.Marshal(pkt)
	if err != nil {
		panic(err)
	}

	return ws.conn.Write(ctx, websocket.MessageBinary, data)
}
