package server

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"nhooyr.io/websocket"
)

// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L1762
// for the interface we need to implement
//
// https://github.com/fullstorydev/grpchan/blob/9b5ad76b6f3d9146862d71690ea87eb9435acc4e/internal/transport_stream.go#L11
// for an example
type serverTransportStream struct {
}

func (*serverTransportStream) Method() string {
	log.Panic("todo")
	return ""
}
func (*serverTransportStream) SetHeader(md metadata.MD) error {
	log.Panic("todo")
	return nil
}
func (*serverTransportStream) SendHeader(md metadata.MD) error {
	log.Panic("todo")
	return nil
}
func (*serverTransportStream) SetTrailer(md metadata.MD) error {
	log.Panic("todo")
	return nil
}

// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L110
type serviceInfo struct {
	// Contains the implementation for the methods in this service.
	serviceImpl interface{}
	methods     map[string]*grpc.MethodDesc
	streams     map[string]*grpc.StreamDesc
	mdata       interface{}
}

type server struct {

	// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L137
	services map[string]*serviceInfo // service name -> service info
}

func parseRawMethod(sm string) (string, string, error) {
	if sm != "" && sm[0] == '/' {
		sm = sm[1:]
	}
	pos := strings.LastIndex(sm, "/")
	if pos == -1 {
		return "", "", fmt.Errorf("invalid method name %s", sm)
	}
	service := sm[:pos]
	method := sm[pos+1:]
	return service, method, nil
}

// See grpc.ServiceRegistrar
func (s *server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	// FIXME: copy-paste from grpc code

	// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L668
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			log.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
		}
	}

	// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L685
	if _, ok := s.services[sd.ServiceName]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	info := &serviceInfo{
		serviceImpl: ss,
		methods:     make(map[string]*grpc.MethodDesc),
		streams:     make(map[string]*grpc.StreamDesc),
		mdata:       sd.Metadata,
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		info.methods[d.MethodName] = d
	}
	for i := range sd.Streams {
		d := &sd.Streams[i]
		info.streams[d.StreamName] = d
	}
	s.services[sd.ServiceName] = info
}

func (s *server) processUnaryRPC(srv *serviceInfo, md *grpc.MethodDesc, rpc *rpcheader.Rpc) *rpcheader.Rpc {
	origCtx := context.Background()

	// The transport stream is needed for the grpc public functions SetHeader(), SetTrailer, etc.
	// At the time of writing we don't use these ourselves so we don't need this immediately.
	sts := &serverTransportStream{}
	ctx := grpc.NewContextWithServerTransportStream(origCtx, sts)

	body := rpc.GetBody()

	codec := encoding.GetCodec(proto.Name)

	dec := func(msg interface{}) error {
		if body.Data == nil {
			return nil
		}

		if err := codec.Unmarshal(body.Data, msg); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		return nil
	}

	resp, appErr := md.Handler(
		srv.serviceImpl, // srv interface{}
		ctx,             // ctx.Context
		dec,             // func(interface{}) error
		nil,             // TODO: UnaryServerInterceptor
	)

	var respStatus *rpcheader.ResponseStatus

	if appErr != nil {
		// https://github.com/grpc/grpc-go/blob/v1.50.0/server.go#L1320
		appStatus, ok := status.FromError(appErr)
		if !ok {
			// Convert non-status application error to a status error with code
			// Unknown, but handle context errors specifically.
			appStatus = status.FromContextError(appErr)
			//appErr = appStatus.Err()
		}

		respStatus = &rpcheader.ResponseStatus{
			Code:    appStatus.Proto().Code,
			Message: appStatus.Proto().Message,
			Details: appStatus.Proto().Details,
		}
	}

	var respBody *rpcheader.Body

	if resp != nil {
		data, err := codec.Marshal(resp)

		if err == nil {
			respBody = &rpcheader.Body{
				Data: data,
			}
		}
	}

	return &rpcheader.Rpc{
		Id:     rpc.GetId(),
		Status: respStatus,
		Body:   respBody,
	}
}

func (s *server) processStreamingRPC(srv *serviceInfo, md *grpc.StreamDesc) {

}

func (s *server) ServeWebsocket(conn *websocket.Conn) {
	ctx := context.Background()

	for {
		log.Printf("server loop: read")

		typ, data, err := conn.Read(ctx)

		if err != nil {
			log.Printf("read err %v", err)
			return
		}

		if typ != websocket.MessageBinary {
			log.Printf("not binary")
			return
		}

		codec := encoding.GetCodec(proto.Name)

		var rpc rpcheader.Rpc

		err = codec.Unmarshal(data, &rpc)
		if err != nil {
			log.Fatalf("error unmarshalling, %v", err)
			return
		}

		if rpc.GetHeader() == nil {
			log.Printf("no header")
			return
		}

		rawMethod := rpc.GetHeader().Method
		service, method, err := parseRawMethod(rawMethod)

		if err != nil {
			log.Printf("error parsing; %v", err)
			return
		}

		log.Printf("got rpc req for: %s / %s", service, method)

		// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L1710
		srv, knownService := s.services[service]
		if knownService {
			if md, ok := srv.methods[method]; ok {
				resp := s.processUnaryRPC(srv, md, &rpc)
				data, err := codec.Marshal(resp)
				if err != nil {
					log.Fatal("todo")
				}
				conn.Write(ctx, websocket.MessageBinary, data)
				continue
			}
			if sd, ok := srv.streams[method]; ok {
				s.processStreamingRPC(srv, sd)
				continue
			}
		}
	}
}

func NewServer() *server {
	s := &server{
		services: make(map[string]*serviceInfo),
	}

	return s
}
