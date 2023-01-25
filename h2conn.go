package goat

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/posener/h2conn"
	"google.golang.org/protobuf/proto"
)

type goatOverH2Conn struct {
	conn *h2conn.Conn

	readBuffer struct {
		sync.Mutex

		bytes    []byte
		capacity int
	}
}

func NewGoatOverH2Conn(conn *h2conn.Conn) *goatOverH2Conn {
	readBufferSize := 1024

	h := &goatOverH2Conn{
		conn: conn,
	}
	h.readBuffer.bytes = make([]byte, readBufferSize)
	h.readBuffer.capacity = readBufferSize

	return h
}

func (h *goatOverH2Conn) Read(ctx context.Context) (*Rpc, error) {
	ok := h.readBuffer.TryLock()
	if !ok {
		err := errors.New("goatOverH2Conn.Read: failed to acquire lock")
		return nil, err
	}
	defer h.readBuffer.Unlock()

	n, err := h.conn.Read(h.readBuffer.bytes)
	if err != nil {
		err := errors.Wrap(err, "goatOverH2Conn.Read: failed to Read")
		return nil, err
	}

	var rpc Rpc
	err = proto.Unmarshal(h.readBuffer.bytes[:n], &rpc)
	if err != nil {
		return nil, errors.Wrap(err, "goatOverH2Conn.Read: failed to unmarshal")
	}

	return &rpc, nil
}

func (h *goatOverH2Conn) Write(ctx context.Context, pkt *Rpc) error {
	data, err := proto.Marshal(pkt)
	if err != nil {
		return errors.Wrap(err, "goatOverH2Conn.Write: failed to marshal")
	}

	_, err = h.conn.Write(data)
	if err != nil {
		return errors.Wrap(err, "goatOverH2Conn.Write: failed to write")
	}

	return nil
}
