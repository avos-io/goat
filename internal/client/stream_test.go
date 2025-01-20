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

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal"
)

var errTest = errors.New("EXPECTED TEST ERROR")

func expectRstStream(t *testing.T, rw *mocks.MockRpcReadWriter, id uint64) {
	rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
		func(rpc *goatorepo.Rpc) bool {
			return rpc.GetId() == id &&
				rpc.GetReset_().GetType() == "RST_STREAM"
		},
	)).Return(nil)
}

func TestLifecycle(t *testing.T) {
	rw := mocks.NewMockRpcReadWriter(t)
	rw.EXPECT().Read(mock.Anything).Return(nil, errTest)

	teardownCalled := make(chan struct{})

	NewStream(context.Background(), 0, "", rw, func() {
		teardownCalled <- struct{}{}
	}, "src", "dst", nil, time.Time{})

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

		sent := &goatorepo.RequestHeader{
			Method: "foo",
			Headers: []*goatorepo.KeyValue{
				{Key: "1", Value: "one"},
				{Key: "2", Value: "two"},
			},
		}

		rw := mocks.NewMockRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&goatorepo.Rpc{
			Id:     1,
			Header: sent,
		}, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(&goatorepo.Rpc{
			Id:      1,
			Header:  &goatorepo.RequestHeader{Method: "foo"},
			Trailer: &goatorepo.Trailer{},
		}, nil).Once()

		stream := NewStream(context.Background(), 1, "", rw, func() {}, "src", "dst", nil, time.Time{})

		got, err := stream.Header()
		is.NoError(err)

		exp, err := internal.ToMetadata(sent.Headers)
		is.NoError(err)
		is.Equal(exp, got)
	})

	t.Run("Read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(nil, errTest)

		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst", nil, time.Time{})

		got, err := stream.Header()
		is.Error(err)
		is.Nil(got)
	})
}

func TestTrailer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		is := require.New(t)

		sent := &goatorepo.Trailer{
			Metadata: []*goatorepo.KeyValue{
				{Key: "0", Value: "jan"},
				{Key: "1", Value: "feb"},
				{Key: "2", Value: "mar"},
			},
		}

		rw := mocks.NewMockRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&goatorepo.Rpc{
			Id:      9,
			Header:  &goatorepo.RequestHeader{Method: "method"},
			Trailer: sent,
		}, nil)

		stream := NewStream(context.Background(), 9, "method", rw, func() {}, "src", "dst", nil, time.Time{})

		err := stream.RecvMsg(nil)
		is.Equal(io.EOF, err)

		exp, err := internal.ToMetadata(sent.Metadata)
		is.NoError(err)
		is.Equal(exp, stream.Trailer())
	})

	t.Run("No metadata", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		rw.EXPECT().Read(mock.Anything).Return(&goatorepo.Rpc{
			Id:      9001,
			Header:  &goatorepo.RequestHeader{Method: "my_method"},
			Trailer: &goatorepo.Trailer{},
		}, nil)

		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst", nil, time.Time{})

		err := stream.RecvMsg(nil)
		is.Equal(io.EOF, err)

		is.Equal(metadata.MD(nil), stream.Trailer())
	})
}

func TestCloseSend(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst", nil, time.Time{})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
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

		rw := mocks.NewMockRpcReadWriter(t)
		stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst", nil, time.Time{})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)
		is.Error(stream.CloseSend())
		close(unblockRead)
	})
}

func TestContext(t *testing.T) {
	rw := mocks.NewMockRpcReadWriter(t)
	unblockRead := make(chan time.Time)
	rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()
	stream := NewStream(context.Background(), 0, "", rw, func() {}, "src", "dst", nil, time.Time{})
	require.NotNil(t, stream.Context())
	unblockRead <- time.Now()
}

func TestSendMsg(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		body := testproto.Msg{Value: 42}
		bodyBytes, err := encoding.GetCodecV2(proto.Name).Marshal(&body)
		is.NoError(err)

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.MatchedBy(
			func(rpc *goatorepo.Rpc) bool {
				return rpc.GetId() == id &&
					rpc.GetHeader().GetMethod() == method &&
					rpc.GetStatus().GetCode() == int32(codes.OK) &&
					assert.Equal(t, bodyBytes.Materialize(), rpc.GetBody().GetData()) &&
					rpc.GetTrailer() == nil
			},
		)).Return(nil)

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst", nil, time.Time{})

		is.NoError(stream.SendMsg(&body))
		unblockRead <- time.Now()
	})

	t.Run("Write err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		teardownCalled := false
		stream := NewStream(context.Background(), id, method, rw, func() {
			teardownCalled = true
		}, "src", "dst", nil, time.Time{})

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest).Maybe()

		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(errTest)

		is.Error(stream.SendMsg(&testproto.Msg{Value: 42}))
		is.True(teardownCalled)

		unblockRead <- time.Now()
	})

	t.Run("Write picks up loop read err", func(t *testing.T) {
		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		rw.EXPECT().Read(mock.Anything).Return(nil, errTest)
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Maybe()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		stream := NewStream(ctx, id, method, rw, func() {}, "src", "dst", nil, time.Time{})

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
		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		recvErr := goatorepo.Rpc{
			Id:     id,
			Header: &goatorepo.RequestHeader{Method: method},
			Status: &goatorepo.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &goatorepo.Trailer{},
		}

		rw.EXPECT().Read(mock.Anything).Return(&recvErr, nil).Once()
		rw.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).Maybe()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		stream := NewStream(ctx, id, method, rw, func() {}, "src", "dst", nil, time.Time{})

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

		rw := mocks.NewMockRpcReadWriter(t)

		msg := testproto.Msg{Value: 9001}
		msgBytes, err := encoding.GetCodecV2(proto.Name).Marshal(&msg)
		is.NoError(err)

		rpc := &goatorepo.Rpc{
			Id: 42,
			Header: &goatorepo.RequestHeader{
				Method: "method",
			},
			Body: &goatorepo.Body{
				Data: msgBytes.Materialize(),
			},
		}
		tr := &goatorepo.Rpc{
			Id: 42,
			Header: &goatorepo.RequestHeader{
				Method: "method",
			},
			Trailer: &goatorepo.Trailer{},
		}

		rw.EXPECT().Read(mock.Anything).Return(rpc, nil).Once()
		rw.EXPECT().Read(mock.Anything).Return(tr, nil)

		stream := NewStream(context.Background(), 42, "method", rw, func() {}, "src", "dst", nil, time.Time{})

		var got testproto.Msg
		is.NoError(stream.RecvMsg(&got))
		is.Equal(msg.Value, got.Value)

		is.Equal(io.EOF, stream.RecvMsg(&got))
	})

	t.Run("RecvMsg picks up loop read err", func(t *testing.T) {
		is := require.New(t)

		rw := mocks.NewMockRpcReadWriter(t)
		id := uint64(9001)
		method := "method"

		stream := NewStream(context.Background(), id, method, rw, func() {}, "src", "dst", nil, time.Time{})

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

		rw := mocks.NewMockRpcReadWriter(t)
		stream := NewStream(context.Background(), 42, "method", rw, func() {}, "src", "dst", nil, time.Time{})

		recvErr := goatorepo.Rpc{
			Id:     42,
			Header: &goatorepo.RequestHeader{Method: "method"},
			Status: &goatorepo.ResponseStatus{
				Code:    int32(codes.Internal),
				Message: codes.Internal.String(),
			},
			Trailer: &goatorepo.Trailer{},
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

func TestResetStream(t *testing.T) {
	rw := mocks.NewMockRpcReadWriter(t)
	ctx, cancel := context.WithCancel(context.Background())
	rw.EXPECT().Read(mock.Anything).RunAndReturn(func(ctx context.Context) (*goatorepo.Rpc, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	expectRstStream(t, rw, 0)

	teardownCalled := make(chan struct{})

	NewStream(ctx, 0, "", rw, func() {
		teardownCalled <- struct{}{}
	}, "src", "dst", nil, time.Time{})

	cancel()

	select {
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	case <-teardownCalled:
		return
	}
}
