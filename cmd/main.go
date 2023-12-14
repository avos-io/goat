package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/avos-io/goat"
	"github.com/avos-io/goat/gen/testproto"
	"google.golang.org/grpc/metadata"
	"nhooyr.io/websocket"
)

func Sam(srv *goat.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			Subprotocols: []string{"echo"},
		})
		if err != nil {
			return
		}
		// defer c.CloseNow()

		//if c.Subprotocol() != "echo" {
		//	c.Close(websocket.StatusPolicyViolation, "client must speak the echo subprotocol")
		//	return
		//}

		rrw := goat.NewGoatOverWebsocket(c)

		srv.Serve(rrw)

		c.Close(websocket.StatusAbnormalClosure, "xxx")
	}
}

type testService struct {
	testproto.UnimplementedTestServiceServer
}

func (testService) BidiStream(testproto.TestService_BidiStreamServer) error {
	return fmt.Errorf("todo")
}
func (testService) ClientStream(testproto.TestService_ClientStreamServer) error {
	return fmt.Errorf("todo")
}
func (testService) ServerStream(msg *testproto.Msg, srv testproto.TestService_ServerStreamServer) error {
	log.Printf("ServerStream got msg %v\n", msg.Value)

	for i := 0; i < int(msg.Value); i++ {
		srv.Send(&testproto.Msg{Value: int32(i)})
	}

	srv.SetTrailer(metadata.New(map[string]string{"FOOO": "bar"}))

	return fmt.Errorf("todo")
}
func (testService) Unary(context.Context, *testproto.Msg) (*testproto.Msg, error) {
	return &testproto.Msg{Value: 44}, nil
}

func main() {
	goat.NewServer("srv0")
	sam := testproto.Msg{}
	sam.GetValue()

	srv := goat.NewServer("")
	testproto.RegisterTestServiceServer(srv, &testService{})

	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		log.Fatalf("error %v", err)
		return
	}
	log.Printf("listening on http://%v", l.Addr())

	s := &http.Server{
		Handler:      http.HandlerFunc(Sam(srv)),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}

	s.Serve(l)
}
