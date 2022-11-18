package client

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"

	rpcheader "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/internal"
)

var errTest = errors.New("EXPECTED TEST ERROR")

func TestLifecycle(t *testing.T) {
	rw := mocks.NewRpcReadWriter(t)
	rw.EXPECT().Read(mock.Anything).Return(nil, errTest)

	teardownCalled := make(chan struct{})

	newClientStream(context.Background(), 0, "", rw, func() {
		teardownCalled <- struct{}{}
	})

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case <-teardownCalled:
		return
	}
}

func TestHeader(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		sent := &rpcheader.RequestHeader{
			Method: "foo",
			Headers: []*rpcheader.KeyValue{
				{Key: "1", Value: "one"},
				{Key: "2", Value: "two"},
			},
		}

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&rpcheader.Rpc{
			Id:     1,
			Header: sent,
		}, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(&rpcheader.Rpc{
			Id:      1,
			Header:  &rpcheader.RequestHeader{Method: "foo"},
			Trailer: &rpcheader.Trailer{},
		}, nil).Once()

		stream := newClientStream(context.Background(), 1, "", rw, func() {})

		got, err := stream.Header()
		is.NoError(err)

		exp, err := internal.ToMetadata(sent.Headers)
		is.NoError(err)
		is.Equal(exp, got)
	})

	t.Run("Read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(nil, errTest)

		stream := newClientStream(context.Background(), 0, "", rw, func() {})

		got, err := stream.Header()
		is.Error(err)
		is.Nil(got)
	})
}

func TestTrailer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		is := require.New(t)

		sent := &rpcheader.Trailer{
			Metadata: []*rpcheader.KeyValue{
				{Key: "0", Value: "jan"},
				{Key: "1", Value: "feb"},
				{Key: "2", Value: "mar"},
			},
		}

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&rpcheader.Rpc{
			Id:      9,
			Header:  &rpcheader.RequestHeader{Method: "method"},
			Trailer: sent,
		}, nil)

		stream := newClientStream(context.Background(), 9, "method", rw, func() {})

		err := stream.RecvMsg(nil)
		is.Equal(io.EOF, err)

		exp, err := internal.ToMetadata(sent.Metadata)
		is.NoError(err)
		is.Equal(exp, stream.Trailer())
	})

	t.Run("No metadata", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&rpcheader.Rpc{
			Id:      9001,
			Header:  &rpcheader.RequestHeader{Method: "my_method"},
			Trailer: &rpcheader.Trailer{},
		}, nil)

		stream := newClientStream(context.Background(), 0, "", rw, func() {})

		err := stream.RecvMsg(nil)
		is.Equal(io.EOF, err)

		is.Equal(metadata.MD(nil), stream.Trailer())
	})
}

func TestCloseSend(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := newClientStream(context.Background(), id, method, rw, func() {})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == id &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					rpc.GetStatus().GetMessage() == codes.OK.String() &&
					rpc.GetTrailer() != nil
			},
		)).Return(nil)

		is.NoError(stream.CloseSend())
		unblockRead <- time.Now()
	})

	t.Run("Write err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream := newClientStream(context.Background(), 0, "", rw, func() {})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)
		is.Error(stream.CloseSend())
		unblockRead <- time.Now()
	})
}

func TestContext(t *testing.T) {
	rw := mocks.NewRpcReadWriter(t)
	unblockRead := make(chan time.Time)
	rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
	stream := newClientStream(context.Background(), 0, "", rw, func() {})
	require.NotNil(t, stream.Context())
	unblockRead <- time.Now()
}

func TestSendMsg(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := newClientStream(context.Background(), id, method, rw, func() {})

		body := rpcheader.IntMessage{Value: 42}
		bodyBytes, err := encoding.GetCodec(proto.Name).Marshal(&body)
		is.NoError(err)

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *rpcheader.Rpc) bool {
				return rpc.GetId() == id &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					assert.Equal(t, rpc.GetBody().GetData(), bodyBytes) &&
					rpc.GetTrailer() == nil
			},
		)).Return(nil)

		is.NoError(stream.SendMsg(&body))
		unblockRead <- time.Now()
	})

	t.Run("Write err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		teardownCalled := false
		stream := newClientStream(context.Background(), id, method, rw, func() {
			teardownCalled = true
		})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)

		is.Error(stream.SendMsg(&rpcheader.IntMessage{Value: 42}))
		is.True(teardownCalled)

		unblockRead <- time.Now()
	})

	t.Run("Write picks up loop read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := newClientStream(context.Background(), id, method, rw, func() {})

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(nil, errTest).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		)

		<-readDone
		is.Error(stream.SendMsg(&rpcheader.IntMessage{Value: 42}))
		rw.AssertNotCalled(t, "Write")
	})

	t.Run("Write picks up recvd error", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := newClientStream(context.Background(), id, method, rw, func() {})

		recvErr := rpcheader.Rpc{
			Id:     id,
			Header: &rpcheader.RequestHeader{Method: method},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &rpcheader.Trailer{},
		}

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(&recvErr, nil).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		).Once()

		<-readDone
		is.Error(stream.SendMsg(&rpcheader.IntMessage{Value: 42}))
		rw.AssertNotCalled(t, "Write")
	})
}

func TestRecvMsg(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream := newClientStream(context.Background(), 42, "method", rw, func() {})

		msg := rpcheader.IntMessage{Value: 9001}
		msgBytes, err := encoding.GetCodec(proto.Name).Marshal(&msg)
		is.NoError(err)

		rpc := &rpcheader.Rpc{
			Id: 42,
			Header: &rpcheader.RequestHeader{
				Method: "method",
			},
			Body: &rpcheader.Body{
				Data: msgBytes,
			},
		}
		tr := &rpcheader.Rpc{
			Trailer: &rpcheader.Trailer{},
		}

		rw.EXPECT().Read(mock.Anything).Return(rpc, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(tr, nil).Once()

		var got rpcheader.IntMessage
		is.NoError(stream.RecvMsg(&got))
		is.Equal(msg.Value, got.Value)

		is.Equal(io.EOF, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg picks up loop read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := newClientStream(context.Background(), id, method, rw, func() {})

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(nil, errTest).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		)

		<-readDone

		var got rpcheader.IntMessage
		is.Error(stream.RecvMsg(&got))
		rw.AssertNotCalled(t, "Write")
	})

	t.Run("RecvMsg picks up error", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream := newClientStream(context.Background(), 42, "method", rw, func() {})

		recvErr := rpcheader.Rpc{
			Id:     42,
			Header: &rpcheader.RequestHeader{Method: "method"},
			Status: &rpcheader.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &rpcheader.Trailer{},
		}

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(&recvErr, nil).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		).Once()

		<-readDone

		var got rpcheader.IntMessage
		is.Error(stream.RecvMsg(&got))
	})
}
