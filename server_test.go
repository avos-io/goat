package goat

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"

	"github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/gen/testproto"
	"github.com/avos-io/goat/gen/testproto/mocks"
	"github.com/avos-io/goat/internal/testutil"
)

var errTest = errors.New("TEST ERROR (EXPECTED)")

func TestNew(t *testing.T) {
	new := NewServer("s0")
	defer new.Stop()
	require.NotNil(t, new)
}

func TestStop(t *testing.T) {
	srv := NewServer("s0")

	conn := testutil.NewTestConn()

	done := make(chan struct{}, 1)
	go func() {
		srv.Serve(context.Background(), conn)
		done <- struct{}{}
	}()

	srv.Stop()
	waitTimeout(t, done)
}

func TestUnary(t *testing.T) {
	t.Run("We can receive a unary RPC and send out its reply", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer("s0")
		defer srv.Stop()

		id := uint64(99)
		method := "/" + testproto.TestService_ServiceDesc.ServiceName + "/Unary"
		sent := testproto.Msg{Value: 42}
		exp := testproto.Msg{Value: 43}

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(m *testproto.Msg) bool {
				return m.Value == sent.Value
			},
		)).Return(&exp, nil)

		testproto.RegisterTestServiceServer(srv, service)

		conn := testutil.NewTestConn()

		go func() {
			srv.Serve(context.Background(), conn)
		}()

		conn.ReadChan <- testutil.ReadReturn{Rpc: wrapRpc(id, method, &sent, "c0", "s0"), Err: nil}

		select {
		case w := <-conn.WriteChan:
			is.Equal(id, w.GetId())
			is.Equal(method, w.GetHeader().GetMethod())
			is.NotNil(w.GetBody())
			is.Equal(exp.Value, unwrapBody(w).GetValue())
			is.NotNil(w.GetTrailer())
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on writeChan")
		}
	})

	t.Run("If the unary handler returns an error, we wrap that up", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer("s0")
		defer srv.Stop()

		id := uint64(1)
		method := "/" + testproto.TestService_ServiceDesc.ServiceName + "/Unary"
		sent := testproto.Msg{Value: 42}

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().Unary(mock.Anything, mock.MatchedBy(
			func(m *testproto.Msg) bool {
				return m.Value == sent.Value
			},
		)).Return(nil, errTest)

		testproto.RegisterTestServiceServer(srv, service)

		conn := testutil.NewTestConn()

		go func() {
			srv.Serve(context.Background(), conn)
		}()

		conn.ReadChan <- testutil.ReadReturn{Rpc: wrapRpc(id, method, &sent, "c0", "s0"), Err: nil}

		select {
		case w := <-conn.WriteChan:
			is.NotEqual(int32(codes.OK), w.GetStatus().GetCode())
			is.NotEmpty(w.GetStatus().GetMessage())
			is.Equal(id, w.GetId())
			is.Equal(method, w.GetHeader().GetMethod())
		case <-time.After(1 * time.Second):
			t.Fatal("timeout on writeChan")
		}
	})
}

func TestServerStream(t *testing.T) {
	t.Run("We can fulfill a server stream request", func(t *testing.T) {
		is := require.New(t)

		srv := NewServer("s0")
		defer srv.Stop()

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/ServerStream"
		sent := testproto.Msg{Value: 1}

		expected := make([]*testproto.Msg, 10)
		for i := range expected {
			expected[i] = &testproto.Msg{Value: int32(i + 1)}
		}

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().ServerStream(mock.Anything, mock.Anything).
			Run(
				func(m *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
					assert.Equal(t, sent.Value, m.Value)
					for _, exp := range expected {
						stream.Send(exp)
					}
				},
			).
			Return(nil)

		testproto.RegisterTestServiceServer(srv, service)

		conn := testutil.NewTestConn()

		go func() {
			srv.Serve(context.Background(), conn)
		}()

		// Open stream
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
			},
			Err: nil,
		}

		// SendMsg
		conn.ReadChan <- testutil.ReadReturn{Rpc: wrapRpc(id, method, &sent, "c0", "s0"), Err: nil}

		// CloseSend
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Status: &goatorepo.ResponseStatus{
					Code:    int32(codes.OK),
					Message: codes.OK.String(),
				},
				Trailer: &goatorepo.Trailer{},
			},
			Err: nil,
		}

		// Read off replies
		exp := 1
		for {
			select {
			case got := <-conn.WriteChan:
				if exp <= len(expected) {
					is.Equal(int32(exp), unwrapBody(got).GetValue())
					exp++
				} else {
					is.NotNil(got.Trailer)
					return
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout")
			}
		}
	})

	t.Run("We send RST stream if we receive a broken stream", func(t *testing.T) {
		is := require.New(t)

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/ServerStream"
		src := "src"
		dst := "dst"

		srv := NewServer(dst)
		defer srv.Stop()
		testproto.RegisterTestServiceServer(srv, mocks.NewMockTestServiceServer(t))
		conn := testutil.NewTestConn()

		go func() {
			srv.Serve(context.Background(), conn)
		}()

		// Just start sending data with no 'open stream'-- this indicates a broken
		// stream, most likely a client 'reconnecting' to a different server than
		// the one which it previously had a running stream with.
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      src,
					Destination: dst,
				},
				Body: &goatorepo.Body{Data: []byte{'u', 'h', 'o', 'h'}},
			},
			Err: nil,
		}

		select {
		case reply := <-conn.WriteChan:
			is.Equal(id, reply.GetId())
			is.Equal(dst, reply.GetHeader().GetSource())
			is.Equal(src, reply.GetHeader().GetDestination())
			is.Equal("RST_STREAM", reply.GetReset_().GetType())
		case <-time.After(1 * time.Second):
			t.Fatal("timeout")
		}
	})

	t.Run("We handle a client abort of a server stream", func(t *testing.T) {
		srv := NewServer("s0")
		defer srv.Stop()

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/ServerStream"
		sent := testproto.Msg{Value: 1}

		rpcDone := make(chan struct{}, 1)
		serverDone := make(chan error, 1)

		service := mocks.NewMockTestServiceServer(t)
		service.EXPECT().ServerStream(mock.Anything, mock.Anything).
			Run(
				func(m *testproto.Msg, stream testproto.TestService_ServerStreamServer) {
					assert.Equal(t, sent.Value, m.Value)
					rpcDone <- <-stream.Context().Done()
				},
			).
			Return(nil)

		testproto.RegisterTestServiceServer(srv, service)

		conn := testutil.NewTestConn()

		go func() {
			serverDone <- srv.Serve(context.Background(), conn)
		}()

		// Open stream
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
			},
			Err: nil,
		}

		// SendMsg
		conn.ReadChan <- testutil.ReadReturn{Rpc: wrapRpc(id, method, &sent, "c0", "s0"), Err: nil}

		// CloseSend
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Status: &goatorepo.ResponseStatus{
					Code:    int32(codes.OK),
					Message: codes.OK.String(),
				},
				Trailer: &goatorepo.Trailer{},
			},
			Err: nil,
		}

		// Reset
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Reset_: &goatorepo.Reset{
					Type: "RST_STREAM",
				},
			},
		}

		// Reset should mean the ServerStream completes
		<-rpcDone

		conn.ReadChan <- testutil.ReadReturn{
			Err: fmt.Errorf("read"),
		}
		<-serverDone
	})

	t.Run("We handle extraneous and erroneous messages on a channel", func(t *testing.T) {
		srv := NewServer("s0")
		defer srv.Stop()

		id := uint64(99)
		method := testproto.TestService_ServiceDesc.ServiceName + "/ServerStream"

		serverDone := make(chan error, 1)

		service := mocks.NewMockTestServiceServer(t)

		testproto.RegisterTestServiceServer(srv, service)

		conn := testutil.NewTestConn()

		go func() {
			serverDone <- srv.Serve(context.Background(), conn)
		}()

		go func() {
			for {
				<-conn.WriteChan
			}
		}()

		// Send trailer for an ID that isn't open
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Trailer: &goatorepo.Trailer{},
			},
			Err: nil,
		}

		id++

		// Send a start streaming with a body - it's invalid, the initial message should have no body
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Body: &goatorepo.Body{},
			},
			Err: nil,
		}

		id++

		// Reset something that doesn't exist
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "s0",
				},
				Reset_: &goatorepo.Reset{
					Type: "RST_STREAM",
				},
			},
			Err: nil,
		}

		// Invalid destination
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      method,
					Source:      "c0",
					Destination: "no such destination",
				},
			},
			Err: nil,
		}

		// Unknown service
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      "no such service",
					Source:      "c0",
					Destination: "s0",
				},
			},
			Err: nil,
		}

		// Unknown method
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
				Header: &goatorepo.RequestHeader{
					Method:      testproto.TestService_ServiceDesc.ServiceName + "/no such method",
					Source:      "c0",
					Destination: "s0",
				},
			},
			Err: nil,
		}

		id++

		// No header
		conn.ReadChan <- testutil.ReadReturn{
			Rpc: &goatorepo.Rpc{
				Id: id,
			},
			Err: nil,
		}

		// Close the Serve down
		conn.ReadChan <- testutil.ReadReturn{
			Err: fmt.Errorf("read"),
		}
		<-serverDone
	})
}

func TestParseGrpcTimeout(t *testing.T) {
	is := require.New(t)

	parse := func(timeout string) time.Duration {
		dur, ok := parseGrpcTimeout(timeout)
		is.True(ok)
		return dur
	}

	is.Equal(time.Duration(4)*time.Hour, parse("4H"))
	is.Equal(time.Duration(4)*time.Minute, parse("4M"))
	is.Equal(time.Duration(4)*time.Second, parse("4S"))
	is.Equal(time.Duration(4)*time.Millisecond, parse("4m"))
	is.Equal(time.Duration(4)*time.Microsecond, parse("4u"))
	is.Equal(time.Duration(4)*time.Nanosecond, parse("4n"))

	checkFail := func(timeout string) {
		_, ok := parseGrpcTimeout(timeout)
		is.False(ok)
	}
	checkFail("4X")
	checkFail("")
	checkFail("well this won't work")
}

func TestParseRawMethod(t *testing.T) {
	is := require.New(t)

	tests := []struct {
		input   string
		service string
		method  string
	}{
		{"myservice.TestService/Unary", "myservice.TestService", "Unary"},
		{"/myservice.TestService/Unary", "myservice.TestService", "Unary"},
		{"myservice/Unary", "myservice", "Unary"},
		{"/myservice/Unary", "myservice", "Unary"},
		{"/a.b/c", "a.b", "c"},
	}

	for _, tt := range tests {
		service, method, err := parseRawMethod(tt.input)
		is.NoError(err, tt.input)
		is.Equal(tt.service, service)
		is.Equal(tt.method, method)
	}

	expectedFails := []string{
		"",
		"a",
		"aaa",
		"\n",
	}
	for _, tt := range expectedFails {
		_, _, err := parseRawMethod(tt)
		is.Error(err, tt)
	}
}

func waitTimeout(t *testing.T, on chan struct{}) {
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("TIMEOUT")
	case <-on:
		return
	}
}

func wrapRpc(id uint64, fullMethod string, msg *testproto.Msg, src, dst string) *goatorepo.Rpc {
	codec := encoding.GetCodec(proto.Name)

	body, err := codec.Marshal(msg)
	if err != nil {
		panic(err)
	}

	rpc := &goatorepo.Rpc{
		Id: id,
		Header: &goatorepo.RequestHeader{
			Method:      fullMethod,
			Source:      src,
			Destination: dst,
		},
		Body: &goatorepo.Body{Data: body},
	}
	return rpc
}

func unwrapBody(rpc *goatorepo.Rpc) *testproto.Msg {
	codec := encoding.GetCodec(proto.Name)

	if rpc.GetBody() == nil {
		return nil
	}

	var out testproto.Msg
	err := codec.Unmarshal(rpc.Body.GetData(), &out)
	if err != nil {
		panic(err)
	}
	return &out
}
