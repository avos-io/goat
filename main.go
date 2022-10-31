package main

// ref: https://github.com/improbable-eng/grpc-web/pull/1084/files
// ref: https://github.com/fullstorydev/grpchan/blob/9b5ad76b6f3d9146862d71690ea87eb9435acc4e/httpgrpc/io.go#L119

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/avos-io/grpc-websockets/internal/client"
	"github.com/avos-io/grpc-websockets/internal/server"

	"nhooyr.io/websocket"
)

type TestService struct {
	rpcheader.UnimplementedIntServiceServer
}

func (*TestService) Add(context.Context, *rpcheader.AddRequest) (*rpcheader.IntMessage, error) {

	return &rpcheader.IntMessage{
		Value: 1,
	}, nil

	// return nil, fmt.Errorf("hello world")
}

func main() {
	srv := server.NewServer()

	ts := &TestService{}

	rpcheader.RegisterIntServiceServer(srv, ts)

	if true {
		fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := websocket.Accept(w, r, nil)
			if err != nil {
				log.Println(err)
				return
			}
			defer c.Close(websocket.StatusInternalError, "the sky is falling")

			_, cancel := context.WithTimeout(r.Context(), time.Second*10)
			defer cancel()

			srv.ServeWebsocket(c)

			c.Close(websocket.StatusNormalClosure, "")
		})

		listener, err := net.Listen("tcp", ":8080")

		go func() {
			err = http.Serve(listener, fn)
			log.Fatal(err)
		}()
	}

	if true {
		c, _, err := websocket.Dial(context.Background(), "ws://localhost:8080", nil)
		if err != nil {
			panic(err)
		}
		defer c.Close(websocket.StatusInternalError, "the sky is falling")

		cci := client.NewWebsocketClientConn(c)
		isc := rpcheader.NewIntServiceClient(cci)

		log.Printf("about to add")

		resp, err := isc.Add(context.Background(), &rpcheader.AddRequest{A: 1, B: 2})

		log.Printf("Add returns: %v || %v\n", resp, err)

		c.Close(websocket.StatusNormalClosure, "")
	}

	// wcc := &WebsocketClientConn{}

}
