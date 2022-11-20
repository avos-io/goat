package server

import (
	"context"
	"errors"
	"io"
	"testing"

	rpcheader "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var errTest = errors.New("EXPECTED TEST ERROR")

func TestContext(t *testing.T) {
	is := require.New(t)

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"foo": "1"}),
	)
	rw := mocks.NewRpcReadWriter(t)
	stream, err := newServerStream(ctx, 0, "", rw)
	is.NoError(err)
	is.Equal(ctx, stream.Context())

	newCtx := metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"bar": "1"}),
	)
	stream.SetContext(newCtx)
	is.NotEqual(ctx, stream.Context())
	is.Equal(newCtx, stream.Context())
}

func TestHeaders(t *testing.T) {
	t.Run("SendHeader", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		val := metadata.New(map[string]string{
			"kev": "bernitz",
			"sam": "jansen",
		})

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					matchMetadata(t, val, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendHeader(val))

		// Can only send once
		is.Error(stream.SendHeader(val))
	})

	t.Run("SendHeader write error", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(context.Background(), 0, "", rw)
		is.NoError(err)

		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)
		is.Equal(errTest, stream.SendHeader(metadata.New(map[string]string{"1": "1"})))
	})

	t.Run("SetHeader then SendHeader", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		val1 := metadata.New(map[string]string{"1": "one"})
		is.NoError(stream.SetHeader(val1))

		rw.AssertNotCalled(t, "Write")

		val2 := metadata.New(map[string]string{"2": "two"})
		is.NoError(stream.SetHeader(val2))

		rw.AssertNotCalled(t, "Write")

		val3 := metadata.New(map[string]string{"3": "three"})

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				merged := val1
				for _, m := range []metadata.MD{val2, val3} {
					for k, v := range m {
						merged[k] = v
					}
				}
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					matchMetadata(t, merged, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendHeader(val3))

		// Can't set after sending
		is.Error(stream.SetHeader(val1))

		// Can only send once
		is.Error(stream.SendHeader(val2))
	})

	t.Run("SendHeader on first SendMsg if not already sent", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		val := metadata.New(map[string]string{"foo": "bar"})
		is.NoError(stream.SetHeader(val))

		rw.AssertNotCalled(t, "Write")

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					matchMetadata(t, val, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendMsg(&rpcheader.IntMessage{}))

		// Can't set after sending
		is.Error(stream.SetHeader(val))

		// Can only send once
		is.Error(stream.SendHeader(val))

		// Headers only sent with first message
		rw.ExpectedCalls = nil
		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					len(rpc.GetHeader().GetHeaders()) == 0
			},
		)).Return(nil)
		is.NoError(stream.SendMsg(&rpcheader.IntMessage{}))
	})

	t.Run("SendHeader on trailer if not already sent", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		val := metadata.New(map[string]string{"foo": "bar"})
		is.NoError(stream.SetHeader(val))

		rw.AssertNotCalled(t, "Write")

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetTrailer() != nil &&
					matchMetadata(t, val, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendTrailer(nil))
	})
}

func TestTrailers(t *testing.T) {
	t.Run("SendTrailer: OK", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		tr1 := metadata.New(map[string]string{"1": "one"})
		stream.SetTrailer(tr1)

		rw.AssertNotCalled(t, "Write")

		tr2 := metadata.New(map[string]string{"2": "two"})
		stream.SetTrailer(tr2)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				merged := tr1
				for k, v := range tr2 {
					merged[k] = v
				}
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					rpc.GetStatus().GetMessage() == codes.OK.String() &&
					matchMetadata(t, merged, rpc.GetTrailer().GetMetadata())
			},
		)).Return(nil)

		is.NoError(stream.SendTrailer(nil))

		// Can only send once
		is.Error(stream.SendTrailer(nil))
	})

	t.Run("SendTrailer: Error", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		tr := metadata.New(map[string]string{"1": "one"})
		stream.SetTrailer(tr)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() != int32(codes.OK) &&
					rpc.GetStatus().GetMessage() != codes.OK.String() &&
					matchMetadata(t, tr, rpc.GetTrailer().GetMetadata())
			},
		)).Return(nil)

		is.NoError(stream.SendTrailer(errTest))
	})

	t.Run("SendTrailer: write fail", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		tr := metadata.New(map[string]string{"1": "one"})
		stream.SetTrailer(tr)

		rw.EXPECT().Write(ctx, mock.Anything).Return(errTest)
		is.Equal(errTest, stream.SendTrailer(errTest))
	})
}

func TestSendMsg(t *testing.T) {
	t.Run("SendMsg", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		codec := encoding.GetCodec(proto.Name)
		m := rpcheader.IntMessage{Value: 42}
		mData, err := codec.Marshal(&m)
		is.NoError(err)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					assert.Equal(t, mData, rpc.GetBody().GetData())
			},
		)).Return(nil)

		is.NoError(stream.SendMsg(&m))
	})

	t.Run("SendMsg write fail", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		rw.EXPECT().Write(ctx, mock.Anything).Return(errTest)

		is.Equal(errTest, stream.SendMsg(&rpcheader.IntMessage{Value: 42}))
	})
}

func TestRecvMsg(t *testing.T) {
	t.Run("RecvMsg OK", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		codec := encoding.GetCodec(proto.Name)
		sent := rpcheader.IntMessage{Value: 42}

		data, err := codec.Marshal(&sent)
		is.NoError(err)

		rpc := rpcheader.Rpc{
			Id: streamId,
			Header: &rpcheader.RequestHeader{
				Method: method,
			},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.OK),
				Message: codes.OK.String(),
			},
			Body: &rpcheader.Body{
				Data: data,
			},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got rpcheader.IntMessage
		is.NoError(stream.RecvMsg(&got))

		is.Equal(sent.GetValue(), got.GetValue())
	})

	t.Run("RecvMsg error", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		rw.EXPECT().Read(ctx).Return(nil, errTest)

		var got rpcheader.IntMessage
		is.Equal(errTest, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg stream end: OK", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		rpc := rpcheader.Rpc{
			Id: streamId,
			Header: &rpcheader.RequestHeader{
				Method: method,
			},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.OK),
				Message: codes.OK.String(),
			},
			Trailer: &rpcheader.Trailer{},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got rpcheader.IntMessage
		is.Equal(io.EOF, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg stream end: Error", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		rw := mocks.NewRpcReadWriter(t)
		stream, err := newServerStream(ctx, streamId, method, rw)
		is.NoError(err)

		rpc := rpcheader.Rpc{
			Id: streamId,
			Header: &rpcheader.RequestHeader{
				Method: method,
			},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &rpcheader.Trailer{},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got rpcheader.IntMessage
		is.Equal(
			status.Error(codes.Internal, codes.Internal.String()),
			stream.RecvMsg(&got),
		)
	})
}

func matchMetadata(t *testing.T, md metadata.MD, kvs []*rpcheader.KeyValue) bool {
	v, err := internal.ToMetadata(kvs)
	if err != nil {
		return false
	}
	// Use assert to get a better test failure message
	return assert.Equal(t, md, v)
}