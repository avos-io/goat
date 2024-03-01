package server

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal"
)

var errTest = errors.New("TEST ERROR (EXPECTED)")

func TestContext(t *testing.T) {
	is := require.New(t)

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.New(map[string]string{"foo": "1"}),
	)
	rw := mocks.NewMockRpcReadWriter(t)
	stream, err := NewServerStream(ctx, 0, "", "", "", rw, nil)
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		val := metadata.New(map[string]string{
			"kev": "bernitz",
			"sam": "jansen",
		})

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
					matchMetadata(t, val, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendHeader(val))

		// Can only send once
		is.Error(stream.SendHeader(val))
	})

	t.Run("SendHeader write error", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(context.Background(), 0, "", "", "", rw, nil)
		is.NoError(err)

		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)
		is.Equal(errTest, stream.SendHeader(metadata.New(map[string]string{"1": "1"})))
	})

	t.Run("SetHeader then SendHeader", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		val1 := metadata.New(map[string]string{"1": "one"})
		is.NoError(stream.SetHeader(val1))

		rw.AssertNotCalled(t, "Write")

		val2 := metadata.New(map[string]string{"2": "two"})
		is.NoError(stream.SetHeader(val2))

		rw.AssertNotCalled(t, "Write")

		val3 := metadata.New(map[string]string{"3": "three"})

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				merged := val1
				for _, m := range []metadata.MD{val2, val3} {
					for k, v := range m {
						merged[k] = v
					}
				}
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		val := metadata.New(map[string]string{"foo": "bar"})
		is.NoError(stream.SetHeader(val))

		rw.AssertNotCalled(t, "Write")

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
					matchMetadata(t, val, rpc.GetHeader().GetHeaders())
			},
		)).Return(nil)

		is.NoError(stream.SendMsg(&testproto.Msg{}))

		// Can't set after sending
		is.Error(stream.SetHeader(val))

		// Can only send once
		is.Error(stream.SendHeader(val))

		// Headers only sent with first message
		rw.ExpectedCalls = nil
		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
					len(rpc.GetHeader().GetHeaders()) == 0
			},
		)).Return(nil)
		is.NoError(stream.SendMsg(&testproto.Msg{}))
	})

	t.Run("SendHeader on trailer if not already sent", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "my_method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		val := metadata.New(map[string]string{"foo": "bar"})
		is.NoError(stream.SetHeader(val))

		rw.AssertNotCalled(t, "Write")

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		tr1 := metadata.New(map[string]string{"1": "one"})
		stream.SetTrailer(tr1)

		rw.AssertNotCalled(t, "Write")

		tr2 := metadata.New(map[string]string{"2": "two"})
		stream.SetTrailer(tr2)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				merged := tr1
				for k, v := range tr2 {
					merged[k] = v
				}
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		tr := metadata.New(map[string]string{"1": "one"})
		stream.SetTrailer(tr)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		codec := encoding.GetCodec(proto.Name)
		m := testproto.Msg{Value: 42}
		mData, err := codec.Marshal(&m)
		is.NoError(err)

		rw.EXPECT().Write(ctx, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == streamId &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetHeader().GetSource() == source &&
					rpc.GetHeader().GetDestination() == destination &&
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
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		rw.EXPECT().Write(ctx, mock.Anything).Return(errTest)

		is.Equal(errTest, stream.SendMsg(&testproto.Msg{Value: 42}))
	})
}

func TestRecvMsg(t *testing.T) {
	t.Run("RecvMsg OK", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		codec := encoding.GetCodec(proto.Name)
		sent := testproto.Msg{Value: 42}

		data, err := codec.Marshal(&sent)
		is.NoError(err)

		rpc := goatorepo.Rpc{
			Id: streamId,
			Header: &goatorepo.RequestHeader{
				Method:      method,
				Source:      destination,
				Destination: source,
			},
			Status: &goatorepo.ResponseStatus{
				Code:    int32(codes.OK),
				Message: codes.OK.String(),
			},
			Body: &goatorepo.Body{
				Data: data,
			},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got testproto.Msg
		is.NoError(stream.RecvMsg(&got))

		is.Equal(sent.GetValue(), got.GetValue())
	})

	t.Run("RecvMsg error", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		rw.EXPECT().Read(ctx).Return(nil, errTest)

		var got testproto.Msg
		is.Contains(stream.RecvMsg(&got).Error(), errTest.Error())
	})

	t.Run("RecvMsg stream end: OK", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		rpc := goatorepo.Rpc{
			Id: streamId,
			Header: &goatorepo.RequestHeader{
				Method:      method,
				Source:      destination,
				Destination: source,
			},
			Status: &goatorepo.ResponseStatus{
				Code:    int32(codes.OK),
				Message: codes.OK.String(),
			},
			Trailer: &goatorepo.Trailer{},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got testproto.Msg
		is.Equal(io.EOF, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg stream end: Error", func(t *testing.T) {
		is := require.New(t)

		ctx := context.Background()
		streamId := uint64(9001)
		method := "method"
		source := "my_source"
		destination := "my_dest"
		rw := mocks.NewMockRpcReadWriter(t)
		stream, err := NewServerStream(ctx, streamId, method, source, destination, rw, nil)
		is.NoError(err)

		rpc := goatorepo.Rpc{
			Id: streamId,
			Header: &goatorepo.RequestHeader{
				Method:      method,
				Source:      destination,
				Destination: source,
			},
			Status: &goatorepo.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &goatorepo.Trailer{},
		}

		rw.EXPECT().Read(ctx).Return(&rpc, nil)

		var got testproto.Msg
		err = stream.RecvMsg(&got)
		is.Error(err)
	})
}

func matchMetadata(t *testing.T, md metadata.MD, kvs []*goatorepo.KeyValue) bool {
	v, err := internal.ToMetadata(kvs)
	if err != nil {
		return false
	}
	// Use assert to get a better test failure message
	return assert.Equal(t, md, v)
}
