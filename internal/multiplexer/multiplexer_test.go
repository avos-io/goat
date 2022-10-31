package multiplexer

import (
	"context"
	"testing"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/status"
	"nhooyr.io/websocket"
)

type testConn struct {
	mock.Mock
}

type readReturn struct {
	typ  websocket.MessageType
	data []byte
	err  error
}

func (c *testConn) Read(ctx context.Context) (websocket.MessageType, []byte, error) {
	args := c.Called(ctx)
	ch := args.Get(0).(chan readReturn)
	rr := <-ch
	return rr.typ, rr.data, rr.err
}

func (c *testConn) Write(ctx context.Context, typ websocket.MessageType, p []byte) error {
	args := c.Called(ctx, typ, p)
	err := args.Error(0)
	return err
}

func makeResponse(id uint64, args interface{}) []byte {
	codec := encoding.GetCodec(proto.Name)

	body, err := codec.Marshal(args)
	if err != nil {
		panic(err)
	}

	ret, err := codec.Marshal(&rpcheader.Rpc{
		Id:   id,
		Body: &rpcheader.Body{Data: body},
	})
	if err != nil {
		panic(err)
	}

	return ret
}

func makeErrorResponse(id uint64, status *rpcheader.ResponseStatus) []byte {
	codec := encoding.GetCodec(proto.Name)

	ret, err := codec.Marshal(&rpcheader.Rpc{
		Id:     id,
		Status: status,
	})
	if err != nil {
		panic(err)
	}

	return ret
}

func TestUnaryMethodSuccess(t *testing.T) {
	tc := &testConn{}

	rm := NewRpcMultiplexer(tc)
	defer rm.Close()

	readChan := make(chan readReturn)

	tc.On("Read", mock.Anything).Return(readChan)
	tc.On("Write", mock.Anything, websocket.MessageBinary, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{websocket.MessageBinary, makeResponse(1, &rpcheader.IntMessage{Value: 42}), nil}
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
	tc.On("Write", mock.Anything, websocket.MessageBinary, mock.Anything).Return(nil).
		Run(func(args mock.Arguments) {
			readChan <- readReturn{websocket.MessageBinary,
				makeErrorResponse(1, &rpcheader.ResponseStatus{Code: int32(codes.InvalidArgument), Message: "Hello world"}), nil}
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
