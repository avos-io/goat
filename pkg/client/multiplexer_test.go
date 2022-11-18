package client

import (
	"context"
	"testing"
	"time"

	rpcheader "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/gen/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"
)

type testConn struct {
	mock.Mock
}

type readReturn struct {
	rpc *rpcheader.Rpc
	err error
}

func (c *testConn) Read(ctx context.Context) (*rpcheader.Rpc, error) {
	args := c.Called(ctx)
	ch := args.Get(0).(chan readReturn)
	rr := <-ch
	return rr.rpc, rr.err
}

func (c *testConn) Write(ctx context.Context, rpc *rpcheader.Rpc) error {
	args := c.Called(ctx, rpc)
	err := args.Error(0)
	return err
}

func makeResponse(id uint64, args interface{}) *rpcheader.Rpc {
	codec := encoding.GetCodec(proto.Name)

	body, err := codec.Marshal(args)
	if err != nil {
		panic(err)
	}

	return &rpcheader.Rpc{
		Id:   id,
		Body: &rpcheader.Body{Data: body},
	}
}

func makeErrorResponse(id uint64, status *rpcheader.ResponseStatus) *rpcheader.Rpc {
	return &rpcheader.Rpc{
		Id:     id,
		Status: status,
	}
}

func TestUnaryMethodSuccess(t *testing.T) {
	tc := &testConn{}

	rm := NewRpcMultiplexer(tc)
	defer rm.Close()

	readChan := make(chan readReturn)

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{makeResponse(1, &rpcheader.IntMessage{Value: 42}), nil}
		})

	valBytes, err := rm.CallUnaryMethod(
		context.Background(),
		&rpcheader.RequestHeader{
			Method: "sam",
		},
		&rpcheader.Body{})

	assert.NoError(t, err)

	codec := encoding.GetCodec(proto.Name)

	var val rpcheader.IntMessage
	codec.Unmarshal(valBytes.Data, &val)

	assert.Equal(t, int32(42), val.Value)
}

func TestUnaryMethodFailure(t *testing.T) {
	tc := &testConn{}

	rm := NewRpcMultiplexer(tc)
	defer rm.Close()

	readChan := make(chan readReturn)

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{
				makeErrorResponse(
					1,
					&rpcheader.ResponseStatus{
						Code:    int32(codes.InvalidArgument),
						Message: "Hello world"},
				),
				nil,
			}
		})

	valBytes, err := rm.CallUnaryMethod(
		context.Background(),
		&rpcheader.RequestHeader{
			Method: "sam",
		},
		&rpcheader.Body{})

	assert.Nil(t, valBytes)
	assert.Error(t, err)

	s, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, s.Code())
	assert.Equal(t, "Hello world", s.Message())
}

func TestNewStreamReadWriter(t *testing.T) {
	t.Run("Write", func(t *testing.T) {
		rw := mocks.NewRpcReadWriter(t)

		unblockRead := make(chan time.Time)
		rw.EXPECT().Read(mock.Anything).WaitUntil(unblockRead).Return(nil, errTest)

		rm := NewRpcMultiplexer(rw)
		defer rm.Close()

		id, srw, teardown := rm.NewStreamReadWriter(context.Background())
		defer teardown()

		ctx := context.Background()
		rpc := &rpcheader.Rpc{
			Id: id,
			Header: &rpcheader.RequestHeader{
				Method: "method",
			},
			Body: &rpcheader.Body{
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

		rm := NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		id, srw, teardown := rm.NewStreamReadWriter(context.Background())
		defer teardown()

		rpc := &rpcheader.Rpc{
			Id: id,
			Header: &rpcheader.RequestHeader{
				Method: "method",
			},
			Body: &rpcheader.Body{
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

		rm := NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		id, srw, teardown := rm.NewStreamReadWriter(context.Background())
		defer teardown()

		readChan <- readReturn{&rpcheader.Rpc{Id: 9001}, nil}
		readChan <- readReturn{&rpcheader.Rpc{Id: 9002}, nil}
		readChan <- readReturn{&rpcheader.Rpc{Id: 9003}, nil}

		rpc := &rpcheader.Rpc{
			Id: id,
			Header: &rpcheader.RequestHeader{
				Method: "method",
			},
			Body: &rpcheader.Body{
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

		rm := NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		_, srw, teardown := rm.NewStreamReadWriter(context.Background())
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

		rm := NewRpcMultiplexer(tc)
		defer rm.Close()

		readChan := make(chan readReturn)
		tc.On("Read", mock.Anything).Return(readChan)

		_, srw, teardown := rm.NewStreamReadWriter(context.Background())

		teardown()

		got, err := srw.Read(context.Background())
		is.Error(err)
		is.Nil(got)
	})
}
