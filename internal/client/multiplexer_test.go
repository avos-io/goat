package client_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"

	"github.com/avos-io/goat/gen/mocks"
	wrapped "github.com/avos-io/goat/gen/protorepo/goat"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal/client"
)

var errTest = errors.New("TEST ERROR (EXPECTED)")

type testConn struct {
	mock.Mock
}

type readReturn struct {
	rpc *wrapped.Rpc
	err error
}

func (c *testConn) Read(ctx context.Context) (*wrapped.Rpc, error) {
	args := c.Called(ctx)
	ch := args.Get(0).(chan readReturn)
	rr := <-ch
	return rr.rpc, rr.err
}

func (c *testConn) Write(ctx context.Context, rpc *wrapped.Rpc) error {
	args := c.Called(ctx, rpc)
	err := args.Error(0)
	return err
}

func makeResponse(id uint64, args interface{}) *wrapped.Rpc {
	codec := encoding.GetCodec(proto.Name)

	body, err := codec.Marshal(args)
	if err != nil {
		panic(err)
	}

	return &wrapped.Rpc{
		Id:   id,
		Body: &wrapped.Body{Data: body},
	}
}

func makeErrorResponse(id uint64, status *wrapped.ResponseStatus) *wrapped.Rpc {
	return &wrapped.Rpc{
		Id:     id,
		Status: status,
	}
}

func TestUnaryMethodSuccess(t *testing.T) {
	tc := &testConn{}

	readChan := make(chan readReturn)

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{makeResponse(1, &testproto.Msg{Value: 42}), nil}
		})

	rm := client.NewRpcMultiplexer(tc)
	defer rm.Close()

	valBytes, err := rm.CallUnaryMethod(
		context.Background(),
		&wrapped.RequestHeader{
			Method: "sam",
		},
		&wrapped.Body{})

	assert.NoError(t, err)

	codec := encoding.GetCodec(proto.Name)

	var val testproto.Msg
	codec.Unmarshal(valBytes.GetData(), &val)

	assert.Equal(t, int32(42), val.Value)
}

func TestUnaryMethodFailure(t *testing.T) {
	tc := &testConn{}

	readChan := make(chan readReturn)

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{
				makeErrorResponse(
					1,
					&wrapped.ResponseStatus{
						Code:    int32(codes.InvalidArgument),
						Message: "Hello world"},
				),
				nil,
			}
		})

	rm := client.NewRpcMultiplexer(tc)
	defer rm.Close()

	valBytes, err := rm.CallUnaryMethod(
		context.Background(),
		&wrapped.RequestHeader{
			Method: "sam",
		},
		&wrapped.Body{})

	assert.Nil(t, valBytes)
	assert.Error(t, err)

	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, s.Code())
	assert.Equal(t, "Hello world", s.Message())
}

// An error status is always an error, even if the resp also has a body.
func TestUnaryMethodFailureDespiteBody(t *testing.T) {
	tc := &testConn{}

	readChan := make(chan readReturn)

	resp := makeResponse(1, &testproto.Msg{Value: 42})
	resp.Status = &wrapped.ResponseStatus{
		Code:    int32(codes.InvalidArgument),
		Message: "My status is telling me no... but my body... my body...",
	}

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{resp, nil}
		})

	rm := client.NewRpcMultiplexer(tc)
	defer rm.Close()

	valBytes, err := rm.CallUnaryMethod(
		context.Background(),
		&wrapped.RequestHeader{
			Method: "yes",
		},
		&wrapped.Body{},
	)

	is := require.New(t)
	is.Nil(valBytes)

	is.Error(err)
	s, ok := status.FromError(err)
	is.True(ok)
	is.Equal(codes.InvalidArgument, s.Code())
	is.Equal(resp.GetStatus().GetMessage(), s.Message())
}

func TestUnaryMethodFailureChannelClosed(t *testing.T) {
	// Make sure we properly handle the case where our IO disconnects while we're
	// waiting on a reply to a unary Rpc.
	rw := mocks.NewRpcReadWriter(t)

	rm := client.NewRpcMultiplexer(rw)
	defer rm.Close()

	waitingOnReply := make(chan time.Time)

	rw.EXPECT().Read(mock.Anything).WaitUntil(waitingOnReply).Return(nil, errTest)
	rw.EXPECT().Write(mock.Anything, mock.Anything).Return(nil).
		Run(func(_a0 context.Context, _a1 *wrapped.Rpc) {
			waitingOnReply <- time.Now()
		})

	ret, err := rm.CallUnaryMethod(
		context.Background(),
		&wrapped.RequestHeader{
			Method: "test",
		},
		&wrapped.Body{},
	)

	is := require.New(t)
	is.Nil(ret)
	is.Error(err)
}

func TestNewStreamReadWriter(t *testing.T) {
	t.Run("Write", func(t *testing.T) {
		rw := mocks.NewRpcReadWriter(t)

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest)

		rm := client.NewRpcMultiplexer(rw)
		defer rm.Close()

		id, srw, teardown, err := rm.NewStreamReadWriter(context.Background())
		require.NoError(t, err)

		defer teardown()

		ctx := context.Background()
		rpc := &wrapped.Rpc{
			Id: id,
			Header: &wrapped.RequestHeader{
				Method: "method",
			},
			Body: &wrapped.Body{
				Data: []byte{1, 2, 3, 4},
			},
		}

		rw.EXPECT().Write(ctx, rpc).Return(nil)
		require.NoError(t, srw.Write(ctx, rpc))

		unblockRead <- time.Now()
	})

	t.Run("Read", func(t *testing.T) {
		is := require.New(t)

		tc := &testConn{}

		rm := client.NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		id, srw, teardown, err := rm.NewStreamReadWriter(context.Background())
		require.NoError(t, err)

		defer teardown()

		rpc := &wrapped.Rpc{
			Id: id,
			Header: &wrapped.RequestHeader{
				Method: "method",
			},
			Body: &wrapped.Body{
				Data: []byte{1, 2, 3, 4},
			},
		}

		readChan <- readReturn{rpc, nil}

		got, err := srw.Read(context.Background())
		is.NoError(err)
		is.Equal(rpc, got)
	})

	t.Run("Read: ignores other Rpcs", func(t *testing.T) {
		is := require.New(t)

		tc := &testConn{}

		rm := client.NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		id, srw, teardown, err := rm.NewStreamReadWriter(context.Background())
		require.NoError(t, err)

		defer teardown()

		readChan <- readReturn{&wrapped.Rpc{Id: 9001}, nil}
		readChan <- readReturn{&wrapped.Rpc{Id: 9002}, nil}
		readChan <- readReturn{&wrapped.Rpc{Id: 9003}, nil}

		rpc := &wrapped.Rpc{
			Id: id,
			Header: &wrapped.RequestHeader{
				Method: "method",
			},
			Body: &wrapped.Body{
				Data: []byte{1, 2, 3, 4},
			},
		}
		readChan <- readReturn{rpc, nil}

		got, err := srw.Read(context.Background())
		is.NoError(err)
		is.Equal(id, got.Id)
	})

	t.Run("Read: breaks on ctx done", func(t *testing.T) {
		is := require.New(t)

		tc := &testConn{}

		rm := client.NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		_, srw, teardown, err := rm.NewStreamReadWriter(context.Background())
		require.NoError(t, err)

		defer teardown()

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			got, err := srw.Read(ctx)
			is.Equal(ctx.Err(), err)
			is.Nil(got)
			done <- struct{}{}
		}()

		cancel()

		select {
		case <-done:
			return
		case <-time.After(1 * time.Second):
			t.Fatal("time out")
		}
	})

	t.Run("Read: breaks after teardown", func(t *testing.T) {
		is := require.New(t)

		tc := &testConn{}

		rm := client.NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		_, srw, teardown, err := rm.NewStreamReadWriter(context.Background())

		require.NoError(t, err)

		teardown()

		got, err := srw.Read(context.Background())
		is.Error(err)
		is.Nil(got)
	})
}
