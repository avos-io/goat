// package e2e contains end-to-end tests
package e2e

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/internal/testutil"
	"github.com/avos-io/goat/pkg/client"
	"github.com/avos-io/goat/pkg/server"
	"github.com/stretchr/testify/require"
)

// TestService is a simple gRPC service for use in testing.
type TestService struct {
	testproto.UnsafeTestServiceServer
}

// An arbitrary bad value which will always cause the server to respond with an error
const badValue = int32(9001)

var errBadValue = errors.New("bad value (expected by test)")

// Increments the number given, or an error on badValue.
func (*TestService) Unary(
	ctx context.Context,
	msg *testproto.Msg,
) (*testproto.Msg, error) {
	v := msg.GetValue()
	if v == badValue {
		return nil, errBadValue
	}
	return &testproto.Msg{Value: v + 1}, nil
}

func TestUnary(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		client, ctx, teardown := setup()
		defer teardown()

		reply, err := client.Unary(ctx, &testproto.Msg{Value: 43})
		is.NoError(err)
		is.NotNil(reply)
		is.Equal(int32(44), reply.Value)
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		client, ctx, teardown := setup()
		defer teardown()

		reply, err := client.Unary(ctx, &testproto.Msg{Value: badValue})
		is.Error(err)
		is.Nil(reply)
	})
}

// Returns all numbers [1, N] or an error on badValue.
func (*TestService) ServerStream(
	msg *testproto.Msg,
	stream testproto.TestService_ServerStreamServer,
) error {
	v := msg.GetValue()
	if v == badValue {
		return errBadValue
	}
	for i := 1; i < int(v); i++ {
		stream.Send(&testproto.Msg{Value: int32(i)})
	}
	return nil
}

func TestServerStream(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		is := require.New(t)

		client, ctx, teardown := setup()
		defer teardown()

		// the server will give us all numbers from 1 to n
		n := int32(10)

		stream, err := client.ServerStream(ctx, &testproto.Msg{Value: n})
		is.NoError(err)
		is.NotNil(stream)

		exp := int32(1)
		for {
			recv, err := stream.Recv()
			if err == io.EOF {
				is.Equal(exp, n)
				return
			}
			is.Equal(exp, recv.GetValue())
			exp++
		}
	})

	t.Run("Error", func(t *testing.T) {
		is := require.New(t)

		client, ctx, teardown := setup()
		defer teardown()

		stream, err := client.ServerStream(ctx, &testproto.Msg{Value: badValue})
		is.NoError(err)
		is.NotNil(stream)

		recv, err := stream.Recv()
		is.Error(err)
		is.Nil(recv)
	})
}

// Returns the increment of any number sent, or an error on badValue.
func (*TestService) BidiStream(stream testproto.TestService_BidiStreamServer) error {
	for i := 0; i < 10; i++ {
		v, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if v.GetValue() == badValue {
			return errBadValue
		}
		err = stream.Send(&testproto.Msg{Value: v.GetValue() + 1})
		if err != nil {
			return err
		}
	}
	return nil
}

// Returns the sum of the numbers given, or err if one of them is badValue.
func (*TestService) ClientStream(stream testproto.TestService_ClientStreamServer) error {
	sum := int32(0)
	for {
		v, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&testproto.Msg{Value: sum})
		}
		if err != nil {
			return err
		}
		sum += v.GetValue()
	}
}

func setup() (testproto.TestServiceClient, context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())

	serverConn := testutil.NewTestConn()
	clientConn := testutil.NewTestConn()

	// server output -> client input
	go func() {
		for {
			select {
			case <-ctx.Done():
				clientConn.ReadChan <- testutil.ReadReturn{Rpc: nil, Err: io.EOF}
				return
			case w := <-serverConn.WriteChan:
				clientConn.ReadChan <- testutil.ReadReturn{Rpc: w, Err: nil}
			}
		}
	}()

	// client output -> server input
	go func() {
		for {
			select {
			case <-ctx.Done():
				serverConn.ReadChan <- testutil.ReadReturn{Rpc: nil, Err: io.EOF}
				return
			case w := <-clientConn.WriteChan:
				serverConn.ReadChan <- testutil.ReadReturn{Rpc: w, Err: nil}
			}
		}
	}()

	server := server.NewServer()
	service := TestService{}
	testproto.RegisterTestServiceServer(server, &service)

	go func() {
		server.Serve(serverConn)
	}()

	client := testproto.NewTestServiceClient(
		client.NewClientConn(clientConn),
	)

	teardown := func() {
		cancel()
		server.Stop()
	}
	return client, ctx, teardown
}
