package goat_test

import (
	"context"
	"testing"
	"time"

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

		demux, ctx, teardown := setupDemuxServer(service, "server0")
		defer teardown()

		client0 := setupClient(demux.IO(), "client0", "server0")
		client1 := setupClient(demux.IO(), "client1", "server0")

		reply0, err := client0.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply0)
		is.Equal(exp.GetValue(), reply0.GetValue())

		reply1, err := client1.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply1)
		is.Equal(exp.GetValue(), reply1.GetValue())
	})

	t.Run("One client disconnects", func(t *testing.T) {
		is := require.New(t)

		send := &testproto.Msg{Value: 42}
		exp := &testproto.Msg{Value: 9001}

		service := mocks.NewTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(msg *testproto.Msg) bool {
				return msg.GetValue() == send.GetValue()
			},
		)).Return(exp, nil)

		demux, ctx, teardown := setupDemuxServer(service, "server1")
		defer teardown()

		client0 := setupClient(demux.IO(), "client0", "server1")
		client1 := setupClient(demux.IO(), "client1", "server1")

		reply0, err := client0.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply0)
		is.Equal(exp.GetValue(), reply0.GetValue())

		reply1, err := client1.Unary(ctx, send)
		is.NoError(err)
		is.NotNil(reply1)
		is.Equal(exp.GetValue(), reply1.GetValue())

		demux.Cancel("client1")
		// There's an unhandled rpc error in multiplexer due to newConnLocked
		// in demux starting a goroutine but not being up before the client invokes
		// the RPC (I think).
		//
		// Same as the sleep in setup below. Something to think about.
		_, err = client1.Unary(ctx, send)
		is.NoError(err)
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
) (*goat.Demux, context.Context, func()) {
	ctx := context.Background()

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
	// TODO: there's a race here which requires a bit of time
	// for the demux goroutine to be setup
	time.Sleep(100 * time.Millisecond)

	teardown := func() {
		demux.Stop()
	}

	return demux, ctx, teardown
}
