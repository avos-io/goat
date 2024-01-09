package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/avos-io/goat"
	"github.com/avos-io/goat/gen/testproto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"nhooyr.io/websocket"
)

type myClientIden string

const myHttpResponseAddr myClientIden = "http-remote-addr"

func WebsocketAcceptor(srv *goat.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"echo"},
		})
		if err != nil {
			return
		}

		log.Printf("%v: connected\n", r.RemoteAddr)

		ctx := context.WithValue(r.Context(), myHttpResponseAddr, r.RemoteAddr)
		rrw := goat.NewGoatOverWebsocket(c)
		err = srv.Serve(ctx, rrw)

		if errors.Is(err, io.EOF) || websocket.CloseStatus(err) == 1000 {
			log.Printf("%v: closed\n", r.RemoteAddr)
		} else {
			log.Printf("%v: unexpected close due to: %v\n", r.RemoteAddr, err)
		}

		c.Close(websocket.StatusAbnormalClosure, err.Error())
	}
}

type testService struct {
	testproto.UnimplementedTestServiceServer
}

func (testService) BidiStream(srv testproto.TestService_BidiStreamServer) error {
	for {
		msg, err := srv.Recv()
		if err != nil {
			break
		}

		for i := 0; i < int(msg.Value); i++ {
			srv.Send(&testproto.Msg{Value: int32(i)})
		}
	}

	return nil
}

func (testService) ClientStream(srv testproto.TestService_ClientStreamServer) error {
	count := 0

	for {
		msg, err := srv.Recv()
		if err != nil {
			break
		}
		count += int(msg.Value)
	}

	srv.SendAndClose(&testproto.Msg{Value: int32(count)})

	return nil
}
func (testService) ServerStream(msg *testproto.Msg, srv testproto.TestService_ServerStreamServer) error {
	for i := 0; i < int(msg.Value); i++ {
		srv.Send(&testproto.Msg{Value: int32(i)})
	}

	srv.SetTrailer(metadata.New(map[string]string{"FOOO": "bar"}))

	return nil
}

func (testService) Unary(ctx context.Context, m *testproto.Msg) (*testproto.Msg, error) {
	trailer := metadata.Pairs(
		"timestamp", time.Now().String(),
		"foo", "bar",
		"input", fmt.Sprintf("%d", m.GetValue()),
	)
	grpc.SetTrailer(ctx, trailer)

	header := metadata.Pairs(
		"foo", "baz",
	)
	grpc.SetHeader(ctx, header)

	return &testproto.Msg{Value: m.GetValue() * 2}, nil
}

func main() {
	srv := goat.NewServer("",
		goat.UnaryInterceptor(
			func(
				ctx context.Context,
				req interface{},
				info *grpc.UnaryServerInfo,
				handler grpc.UnaryHandler,
			) (resp interface{}, err error) {
				clid := ctx.Value(myHttpResponseAddr)
				log.Printf("%v: unary %s\n", clid, info.FullMethod)
				return handler(ctx, req)
			}),
		goat.StreamInterceptor(
			func(
				srv interface{},
				ss grpc.ServerStream,
				info *grpc.StreamServerInfo,
				handler grpc.StreamHandler,
			) error {
				clid := ss.Context().Value(myHttpResponseAddr)
				log.Printf("%v: stream %s\n", clid, info.FullMethod)
				return handler(srv, ss)
			}),
	)
	testproto.RegisterTestServiceServer(srv, &testService{})

	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		log.Fatalf("error %v", err)
		return
	}
	log.Printf("listening on http://%v", l.Addr())

	s := &http.Server{
		Handler:      http.HandlerFunc(WebsocketAcceptor(srv)),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	s.Serve(l)
}
