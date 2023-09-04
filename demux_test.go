package goat_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/avos-io/goat"
	goatorepo "github.com/avos-io/goat/gen/goatorepo"
)

func TestDemuxSource(t *testing.T) {
	t.Run("multiple clients, single request", func(t *testing.T) {
		is := require.New(t)

		r := make(chan *goat.Rpc)
		w := make(chan *goat.Rpc)

		rw := goat.NewGoatOverChannel(r, w)

		conn := make(chan goat.RpcReadWriter)

		demux := goat.NewDemux(
			context.Background(),
			rw,
			func(rpc *goat.Rpc) string {
				return rpc.Header.Source
			},
			func(rrw goat.RpcReadWriter) {
				conn <- rrw
			},
		)
		go demux.Run()
		t.Cleanup(demux.Stop)

		for i := 0; i < 10; i++ {
			client := fmt.Sprintf("client%d", i)
			rpc := &goatorepo.Rpc{
				Id: 1,
				Header: &goatorepo.RequestHeader{
					Source: client,
				},
			}

			r <- rpc

			c := <-conn
			rpc, err := c.Read(context.Background())
			is.NoError(err)
			is.Equal(rpc.GetId(), uint64(1))
			is.Equal(rpc.GetHeader().GetSource(), client)
		}
	})

	t.Run("single client, multiple requests", func(t *testing.T) {
		is := require.New(t)

		r := make(chan *goat.Rpc)
		w := make(chan *goat.Rpc)

		rw := goat.NewGoatOverChannel(r, w)

		conn := make(chan goat.RpcReadWriter)

		demux := goat.NewDemux(
			context.Background(),
			rw,
			func(rpc *goat.Rpc) string {
				return rpc.Header.Source
			},
			func(rrw goat.RpcReadWriter) {
				conn <- rrw
			},
		)
		go demux.Run()
		t.Cleanup(demux.Stop)

		client := "client"
		var c goat.RpcReadWriter

		for i := 1; i < 10; i++ {
			rpc := &goatorepo.Rpc{
				Id: uint64(i),
				Header: &goatorepo.RequestHeader{
					Source: client,
				},
			}

			r <- rpc
			if i == 1 {
				c = <-conn
			}

			rpc, err := c.Read(context.Background())
			is.NoError(err)
			is.Equal(rpc.GetId(), uint64(i))
			is.Equal(rpc.GetHeader().GetSource(), client)
		}
	})
}
