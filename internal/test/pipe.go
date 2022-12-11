package sam

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"

	wrapped "github.com/avos-io/goat/gen"
	"google.golang.org/protobuf/proto"
)

type goatOverPipe struct {
	conn       net.Conn
	readMutex  sync.Mutex
	writeMutex sync.Mutex
}

func newGoatOverPipe(c net.Conn) *goatOverPipe {
	return &goatOverPipe{conn: c}
}

func (g *goatOverPipe) Read(ctx context.Context) (*wrapped.Rpc, error) {
	g.readMutex.Lock()
	defer g.readMutex.Unlock()

	var msgSize uint32
	err := binary.Read(g.conn, binary.BigEndian, &msgSize)
	if err != nil {
		panic(err)
	}

	data := make([]byte, msgSize)
	_, err = io.ReadFull(g.conn, data)
	if err != nil {
		panic(err)
	}

	var rpc wrapped.Rpc
	err = proto.Unmarshal(data, &rpc)
	if err != nil {
		panic(err)
	}

	return &rpc, nil
}

func (g *goatOverPipe) Write(ctx context.Context, pkt *wrapped.Rpc) error {
	g.writeMutex.Lock()
	defer g.writeMutex.Unlock()

	data, err := proto.Marshal(pkt)
	if err != nil {
		panic(err)
	}

	var msgSize uint32 = uint32(len(data))
	err = binary.Write(g.conn, binary.BigEndian, &msgSize)
	if err != nil {
		panic(err)
	}

	_, err = g.conn.Write(data)
	if err != nil {
		panic(err)
	}

	return nil
}
