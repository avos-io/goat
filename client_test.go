package goat

import (
	"context"
	"net"
	"testing"

	grpcStatsMocks "github.com/avos-io/goat/gen/grpc/stats/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

func TestClientStatsHandler(t *testing.T) {
	t.Run("Unary", func(t *testing.T) {
		is := require.New(t)

		statsMock := grpcStatsMocks.NewMockHandler(t)

		// TODO: conn stuff
		expectStatsHandleConn[*stats.ConnBegin](statsMock).Times(1)
		expectStatsHandleConn[*stats.ConnEnd](statsMock).Times(1)

		expectStatsHandleRPC[*stats.Begin](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.InPayload](statsMock).Times(1)
		expectStatsHandleRPC[*stats.End](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutHeader](statsMock).Times(1)
		expectStatsHandleRPC[*stats.OutPayload](statsMock).Times(1)

		// Unary doesn't send a trailer (yet?)
		// expectStatsHandleRPC[*stats.OutTrailer](statsMock).Times(1)

		type ctxValues int
		const (
			ctxValueTagRPC ctxValues = iota
			ctxValueTagConn
		)

		tagRPCCall := statsMock.EXPECT().TagRPC(mock.Anything, mock.Anything)
		tagRPCCall.Run(func(ctx context.Context, tag *stats.RPCTagInfo) {
			ctx2 := metadata.AppendToOutgoingContext(ctx, "from-tag-rpc", "test 1")
			tagRPCCall.Return(ctx2)
		}).Times(1)
		tagConnCall := statsMock.EXPECT().TagConn(mock.Anything, mock.Anything)
		tagConnCall.Run(func(ctx context.Context, _a1 *stats.ConnTagInfo) {
			ctx2 := context.WithValue(ctx, ctxValueTagConn, "TagConn")
			tagConnCall.Return(ctx2)
		}).Times(1)

		srv := NewServer("s0")
		defer srv.Stop()

		exp := testproto.Msg{Value: 43}

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.Anything).
			Run(func(ctx context.Context, msg *testproto.Msg) {
				md, ok := metadata.FromIncomingContext(ctx)
				is.True(ok)
				is.Equal("test 1", md.Get("from-tag-rpc")[0])
				// FIXME:
				//is.Equal("TagConn", ctx.Value(ctxValueTagConn))
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

		ps1.Close()
		ps2.Close()

		<-done
	})
}
