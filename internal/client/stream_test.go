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

	wrapped "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal"
)

var errTest = errors.New("EXPECTED TEST ERROR")

func TestLifecycle(t *testing.T) {
	rw := mocks.NewRpcReadWriter(t)
	rw.EXPECT().Read(mock.Anything).Return(nil, errTest)

	teardownCalled := make(chan struct{})

	NewStream(context.Background(), 0, "", rw, func() {
		teardownCalled <- struct{}{}
	}, "src", "dst")

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

		sent := &wrapped.RequestHeader{
			Method: "foo",
			Headers: []*wrapped.KeyValue{
				{Key: "1", Value: "one"},
				{Key: "2", Value: "two"},
			},
		}

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&wrapped.Rpc{
			Id:     1,
			Header: sent,
		}, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(&wrapped.Rpc{
			Id:      1,
			Header:  &wrapped.RequestHeader{Method: "foo"},
			Trailer: &wrapped.Trailer{},
		}, nil).Once()

		stream := NewStream(context.Background(), 1, "", rw, func() {}, "src", "dst")

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

		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst")

		got, err := stream.Header()
		is.Error(err)
		is.Nil(got)
	})
}

func TestTrailer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		is := require.New(t)

		sent := &wrapped.Trailer{
			Metadata: []*wrapped.KeyValue{
				{Key: "0", Value: "jan"},
				{Key: "1", Value: "feb"},
				{Key: "2", Value: "mar"},
			},
		}

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&wrapped.Rpc{
			Id:      9,
			Header:  &wrapped.RequestHeader{Method: "method"},
			Trailer: sent,
		}, nil)

		stream := NewStream(context.Background(), 9, "method", rw, func() {}, "src", "dst")

		err := stream.RecvMsg(nil)
		is.Equal(io.EOF, err)

		exp, err := internal.ToMetadata(sent.Metadata)
		is.NoError(err)
		is.Equal(exp, stream.Trailer())
	})

	t.Run("No metadata", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&wrapped.Rpc{
			Id:      9001,
			Header:  &wrapped.RequestHeader{Method: "my_method"},
			Trailer: &wrapped.Trailer{},
		}, nil)

		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst")

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

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst")

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *wrapped.Rpc) bool {
				return rpc.GetId() == id &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					rpc.GetStatus().GetMessage() == codes.OK.String() &&
					rpc.GetTrailer() != nil
			},
		)).Return(nil)

		is.NoError(stream.CloseSend())
		close(unblockRead)
	})

	t.Run("Write err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst")

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)
		is.Error(stream.CloseSend())
		close(unblockRead)
	})
}

func TestContext(t *testing.T) {
	rw := mocks.NewRpcReadWriter(t)
	unblockRead := make(chan time.Time)
	rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
	stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst")
	require.NotNil(t, stream.Context())
	unblockRead <- time.Now()
}

func TestSendMsg(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		body := testproto.Msg{Value: 42}
		bodyBytes, err := encoding.GetCodec(proto.Name).Marshal(&body)
		is.NoError(err)

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *wrapped.Rpc) bool {
				return rpc.GetId() == id &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					assert.Equal(t, rpc.GetBody().GetData(), bodyBytes) &&
					rpc.GetTrailer() == nil
			},
		)).Return(nil)

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst")

		is.NoError(stream.SendMsg(&body))
		unblockRead <- time.Now()
	})

	t.Run("Write err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		teardownCalled := false
		stream := NewStream(context.Background(), id, method, rw, func() {
			teardownCalled = true
		}, "src", "dst")

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)

		is.Error(stream.SendMsg(&testproto.Msg{Value: 42}))
		is.True(teardownCalled)

		unblockRead <- time.Now()
	})

	t.Run("Write picks up loop read err", func(t *testing.T) {
		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		rw.EXPECT().Read(mock.Anything).Return(nil, errTest)
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Maybe()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		stream := NewStream(ctx, id, method, rw, func() {}, "src", "dst")

		// blocks until we get our first response, which will be the err
		_, _ = stream.Header()

		// there's a race between the stream entering the error state after
		// receiving an error, and us getting that error when we try to SendMsg, so
		// keep trying with a timeout
		errChan := make(chan error, 1)

		go func() {
			for {
				if ctx.Err() != nil {
					return
				}
				if err := stream.SendMsg(&testproto.Msg{Value: 42}); err != nil {
					errChan <- err
					return
				}
			}
		}()
	})

	t.Run("Write picks up recvd error", func(t *testing.T) {
		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		recvErr := wrapped.Rpc{
			Id:     id,
			Header: &wrapped.RequestHeader{Method: method},
			Status: &wrapped.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &wrapped.Trailer{},
		}

		rw.EXPECT().Read(mock.Anything).Return(&recvErr, nil).Once()
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Maybe()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		stream := NewStream(ctx, id, method, rw, func() {}, "src", "dst")

		// blocks until we get our first response, which will be the err
		_, _ = stream.Header()

		// there's a race between the stream entering the error state after
		// receiving an error, and us getting that error when we try to SendMsg, so
		// keep trying with a timeout
		errChan := make(chan error, 1)

		go func() {
			for {
				if ctx.Err() != nil {
					return
				}
				if err := stream.SendMsg(&testproto.Msg{Value: 42}); err != nil {
					errChan <- err
					return
				}
			}
		}()
	})
}

func TestRecvMsg(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)

		msg := testproto.Msg{Value: 9001}
		msgBytes, err := encoding.GetCodec(proto.Name).Marshal(&msg)
		is.NoError(err)

		rpc := &wrapped.Rpc{
			Id: 42,
			Header: &wrapped.RequestHeader{
				Method: "method",
			},
			Body: &wrapped.Body{
				Data: msgBytes,
			},
		}
		tr := &wrapped.Rpc{
			Id: 42,
			Header: &wrapped.RequestHeader{
				Method: "method",
			},
			Trailer: &wrapped.Trailer{},
		}

		rw.EXPECT().Read(mock.Anything).Return(rpc, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(tr, nil)

		stream := NewStream(context.Background(), 42, "method", rw, func() {}, "src", "dst")

		var got testproto.Msg
		is.NoError(stream.RecvMsg(&got))
		is.Equal(msg.Value, got.Value)

		is.Equal(io.EOF, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg picks up loop read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst")

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(nil, errTest).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		)

		<-readDone

		var got testproto.Msg
		is.Error(stream.RecvMsg(&got))
		rw.AssertNotCalled(t, "Write")
	})

	t.Run("RecvMsg picks up error", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewRpcReadWriter(t)
		stream := NewStream(context.Background(), 42, "method", rw, func() {}, "src", "dst")

		recvErr := wrapped.Rpc{
			Id:     42,
			Header: &wrapped.RequestHeader{Method: "method"},
			Status: &wrapped.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &wrapped.Trailer{},
		}

		readDone := make(chan struct{})
		rw.EXPECT().Read(mock.Anything).Return(&recvErr, nil).Run(
			func(ctx context.Context) { readDone <- struct{}{} },
		).Once()

		<-readDone

		var got testproto.Msg
		is.Error(stream.RecvMsg(&got))
	})
}
