package goat

import (
	"context"
	"net"
	"testing"

	grpcStatsMocks "github.com/avos-io/goat/gen/grpc/stats/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

func TestClientStatsHandler(t *testing.T) {
	t.Run("Unary", func(t *testing.T) {
		is := require.New(t)

		statsMock := grpcStatsMocks.NewMockHandler(t)

		expectStatsHandleConn[*stats.ConnBegin](statsMock).Times(1)
		expectStatsHandleConn[*stats.ConnEnd](statsMock).Times(1)

		expectStatsHandleRPC[*stats.Begin](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InPayload](statsMock).Times(1)
		expectStatsHandleRPC[*stats.End](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutPayload](statsMock).Times(1)

		tagRPCCall := statsMock.EXPECT().TagRPC(mock.Anything, mock.Anything)
		tagRPCCall.Run(func(ctx context.Context, tag *stats.RPCTagInfo) {
			ctx2 := metadata.AppendToOutgoingContext(ctx, "from-tag-rpc", "test 1")
			tagRPCCall.Return(ctx2)
		}).Times(1)
		statsMock.EXPECT().TagConn(mock.Anything, mock.Anything).Times(1).Return(context.Background())

		srv := NewServer("s0")
		defer srv.Stop()

		exp := testproto.Msg{Value: 43}

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.Anything).
			Run(func(ctx context.Context, msg *testproto.Msg) {
				md, ok := metadata.FromIncomingContext(ctx)
				is.True(ok)
				is.Equal("test 1", md.Get("from-tag-rpc")[0])
			}).Times(1).Return(&exp, nil)

		testproto.RegisterTestServiceServer(srv, service)

		ps1, ps2 := net.Pipe()
		done := make(chan struct{})

		go func() {
			srv.Serve(context.Background(), testutil.NewGoatOverPipe(ps1))
			done <- struct{}{}
		}()

		clientConn := NewClientConn(testutil.NewGoatOverPipe(ps2), "c0", "s0", WithStatsHandler(statsMock))
		cl := testproto.NewTestServiceClient(clientConn)

		result, err := cl.Unary(context.Background(), &testproto.Msg{Value: 1})
		is.NoError(err)
		is.Equal(exp.GetValue(), result.GetValue())

		clientConn.Close()

		ps1.Close()
		ps2.Close()

		<-done
	})

	t.Run("streaming", func(t *testing.T) {
		is := require.New(t)

		statsMock := grpcStatsMocks.NewMockHandler(t)

		expectStatsHandleConn[*stats.ConnBegin](statsMock).Times(1)
		expectStatsHandleConn[*stats.ConnEnd](statsMock).Times(1)

		expectStatsHandleRPC[*stats.Begin](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InPayload](statsMock).Times(1)
		expectStatsHandleRPC[*stats.End](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutPayload](statsMock).Times(1)

		tagRPCCall := statsMock.EXPECT().TagRPC(mock.Anything, mock.Anything)
		tagRPCCall.Run(func(ctx context.Context, tag *stats.RPCTagInfo) {
			ctx2 := metadata.AppendToOutgoingContext(ctx, "from-tag-rpc", "test 1")
			tagRPCCall.Return(ctx2)
		}).Times(1)
		statsMock.EXPECT().TagConn(mock.Anything, mock.Anything).Times(1).Return(context.Background())

		srv := NewServer("s0")
		defer srv.Stop()

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().BidiStream(mock.Anything).Run(func(ss testproto.TestService_BidiStreamServer) {
			msg, err := ss.Recv()
			is.NoError(err)

			is.Equal(int32(42), msg.GetValue())

			err = ss.Send(msg)
			is.NoError(err)
		}).Return(nil).Times(1)

		testproto.RegisterTestServiceServer(srv, service)

		ps1, ps2 := net.Pipe()
		done := make(chan struct{})

		go func() {
			srv.Serve(context.Background(), testutil.NewGoatOverPipe(ps1))
			done <- struct{}{}
		}()

		clientConn := NewClientConn(testutil.NewGoatOverPipe(ps2), "c0", "s0", WithStatsHandler(statsMock))
		cl := testproto.NewTestServiceClient(clientConn)

		client, err := cl.BidiStream(context.Background())
		is.NoError(err)

		err = client.Send(&testproto.Msg{Value: 42})
		is.NoError(err)

		msg, err := client.Recv()
		is.NoError(err)

		is.Equal(int32(42), msg.GetValue())

		// This should die - the server will finish the streaming RPC
		_, err = client.Recv()
		is.Error(err)

		clientConn.Close()

		ps1.Close()
		ps2.Close()

		<-done
	})
}

func TestClientResetStream(t *testing.T) {
	is := require.New(t)

	statsMock := grpcStatsMocks.NewMockHandler(t)

	expectStatsHandleConn[*stats.ConnBegin](statsMock).Times(1)
	expectStatsHandleConn[*stats.ConnEnd](statsMock).Times(1)

	expectStatsHandleRPC[*stats.Begin](statsMock).Times(1)
	expectStatsHandleRPC[*stats.End](statsMock).Times(1)
	expectStatsHandleRPC[*stats.OutHeader](statsMock).Times(1)
	expectStatsHandleRPC[*stats.OutPayload](statsMock).Times(1)
	expectStatsHandleRPC[*stats.OutTrailer](statsMock).Times(1)

	tagRPCCall := statsMock.EXPECT().TagRPC(mock.Anything, mock.Anything)
	tagRPCCall.Run(func(ctx context.Context, tag *stats.RPCTagInfo) {
		ctx2 := metadata.AppendToOutgoingContext(ctx, "from-tag-rpc", "test 1")
		tagRPCCall.Return(ctx2)
	}).Times(1)
	statsMock.EXPECT().TagConn(mock.Anything, mock.Anything).Times(1).Return(context.Background())

	srv := NewServer("s0")
	defer srv.Stop()

	srvDoneChan := make(chan struct{})

	service := mocks.NewMockTestServiceServer(t)
	service.EXPECT().ServerStream(mock.Anything, mock.Anything).Run(func(_ *testproto.Msg, ss testproto.TestService_ServerStreamServer) {
		log.Warn().Msg("ServerStream called")
		<-ss.Context().Done()
		log.Warn().Msg("ServerStream done")
		close(srvDoneChan)
	}).Return(nil).Times(1)

	testproto.RegisterTestServiceServer(srv, service)

	ps1, ps2 := net.Pipe()
	done := make(chan struct{})

	go func() {
		srv.Serve(context.Background(), testutil.NewGoatOverPipe(ps1))
		done <- struct{}{}
	}()

	clientConn := NewClientConn(testutil.NewGoatOverPipe(ps2), "c0", "s0", WithStatsHandler(statsMock))
	cl := testproto.NewTestServiceClient(clientConn)

	ctx, cancel := context.WithCancel(context.Background())
	_, err := cl.ServerStream(ctx, &testproto.Msg{Value: 42})
	is.NoError(err)

	cancel()

	// Check that the server context gets cancelled
	<-srvDoneChan

	clientConn.Close()

	ps1.Close()
	ps2.Close()

	<-done
}
