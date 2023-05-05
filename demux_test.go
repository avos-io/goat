// package contains end-to-end tests
package goat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/avos-io/goat"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
)

func TestDemux(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		send := &testproto.Msg{Value: 42}
		exp := &testproto.Msg{Value: 9001}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(msg *testproto.Msg) bool {
				return msg.GetValue() == send.GetValue()
			},
		)).Return(exp, nil)

		rw, ctx, teardown := setupDemuxServer(service, "server0")
		defer teardown()

		client0 := setupClient(rw, "client0", "server0")
		client1 := setupClient(rw, "client1", "server0")

		reply0, err := client0.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply0)
		is.Equal(exp.GetValue(), reply0.GetValue())

		reply1, err := client1.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply1)
		is.Equal(exp.GetValue(), reply1.GetValue())
	})
}

func setupClient(
	srvRw goat.RpcReadWriter, clientAddr, serverAddr string,
) testproto.TestServiceClient {
	return testproto.NewTestServiceClient(
		goat.NewClientConn(srvRw, clientAddr, serverAddr),
	)
}

func setupDemuxServer(
	s testproto.TestServiceServer, serverAddr string,
) (goat.RpcReadWriter, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	server := goat.NewServer(serverAddr)
	testproto.RegisterTestServiceServer(server, s)

	r := make(chan *goat.Rpc)
	w := make(chan *goat.Rpc)

	demux := goat.NewDemux(
		ctx,
		server,
		r,
		w,
		func(rpc *goat.Rpc) string {
			return rpc.Header.Source
		},
	)
	go demux.Run()

	teardown := func() {
		cancel()
		demux.Stop()
	}

	return goat.NewGoatOverChannel(w, r), ctx, teardown
}
