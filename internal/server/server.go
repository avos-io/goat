package server

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	rpcheader "github.com/avos-io/grpc-websockets/gen"
	"github.com/avos-io/grpc-websockets/internal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ServerOption is an option used when constructing a NewServer.
type ServerOption interface {
	apply(*server)
}

type serverOptFunc func(*server)

func (fn serverOptFunc) apply(s *server) {
	fn(s)
}

// UnaryInterceptor returns a ServerOption that sets the UnaryServerInterceptor
// for the server. Only one unary interceptor can be installed. The construction
// of multiple interceptors (e.g., chaining) can be implemented at the caller.
func UnaryInterceptor(i grpc.UnaryServerInterceptor) ServerOption {
	return serverOptFunc(func(s *server) {
		s.unaryInterceptor = i
	})
}

// StreamInterceptor returns a ServerOption that sets the StreamServerInterceptor
// for the server. Only one stream interceptor can be installed.
func StreamInterceptor(i grpc.StreamServerInterceptor) ServerOption {
	return serverOptFunc(func(s *server) {
		s.streamInterceptor = i
	})
}

func NewServer(opts ...ServerOption) *server {
	srv := server{
		services: make(map[string]*serviceInfo),
	}
	for _, opt := range opts {
		opt.apply(&srv)
	}
	return &srv
}

type serviceInfo struct {
	name        string
	serviceImpl interface{}
	methods     map[string]*grpc.MethodDesc
	streams     map[string]*grpc.StreamDesc
	mdata       interface{}
}

type server struct {
	services map[string]*serviceInfo // service name -> service info

	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor
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
			log.Fatalf(
				"grpc: Server.RegisterService found the handler of type %v that does not satisfy %v",
				st,
				ht,
			)
		}
	}

	// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L685
	if _, ok := s.services[sd.ServiceName]; ok {
		log.Fatalf(
			"grpc: Server.RegisterService found duplicate service registration for %q",
			sd.ServiceName,
		)
	}
	info := &serviceInfo{
		name:        sd.ServiceName,
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

func (s *server) Serve(rw internal.RpcReadWriter) {
	handler := newHandler(
		context.Background(),
		s.services,
		s.unaryInterceptor,
		s.streamInterceptor,
		rw,
	)
	for {
		err := handler.serveStep()
		if err != nil {
			log.Printf("handler error: %v", err)
			return
		}
	}
}

// handler for a specific websocket connection
type handler struct {
	ctx context.Context

	// FIXME: we shouldn't have to pass these through
	services          map[string]*serviceInfo // service name -> service info
	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor

	rw    internal.RpcReadWriter
	codec encoding.Codec

	streams map[uint64]chan *rpcheader.Rpc
	mutex   sync.Mutex
}

func newHandler(
	ctx context.Context,
	services map[string]*serviceInfo,
	unaryInterceptor grpc.UnaryServerInterceptor,
	streamInterceptor grpc.StreamServerInterceptor,
	rw internal.RpcReadWriter,
) *handler {
	return &handler{
		ctx:               ctx,
		services:          services,
		unaryInterceptor:  unaryInterceptor,
		streamInterceptor: streamInterceptor,
		rw:                rw,
		codec:             encoding.GetCodec(proto.Name),
		streams:           map[uint64]chan *rpcheader.Rpc{},
	}
}

func (h *handler) serveStep() error {
	log.Printf("server loop: read")

	rpc, err := h.rw.Read(h.ctx)
	if err != nil {
		log.Printf("read err %v", err)
		return err
	}

	if rpc.GetHeader() == nil {
		return fmt.Errorf("no header")
	}

	rawMethod := rpc.GetHeader().Method
	service, method, err := parseRawMethod(rawMethod)
	if err != nil {
		return err
	}

	log.Printf("got rpc req for: %s / %s", service, method)

	// https://github.com/grpc/grpc-go/blob/9127159caf5a3879dad56b795938fde3bc0a7eaa/server.go#L1710
	srv, known := h.services[service]
	if !known {
		log.Printf("unknown service, %s", service)
		return nil
	}
	if md, ok := srv.methods[method]; ok {
		resp := h.processUnaryRpc(srv, md, rpc)
		h.rw.Write(h.ctx, resp)
		return nil
	}
	if sd, ok := srv.streams[method]; ok {
		return h.processStreamingRpc(srv, sd, rpc)
	}
	log.Printf("unhandled method, %s %s", service, method)
	return nil
}

func (h *handler) processUnaryRpc(
	srv *serviceInfo,
	md *grpc.MethodDesc,
	rpc *rpcheader.Rpc,
) *rpcheader.Rpc {
	ctx := context.Background()

	ctx, cancel, err := contextFromHeaders(ctx, rpc.GetHeader())
	if err != nil {
		log.Panicf("failed to processUnaryRpc: %v", err)
	}
	defer cancel()

	fullMethod := fmt.Sprintf("/%s/%s", srv.name, md.MethodName)
	sts := NewUnaryServerTransportStream(fullMethod)
	ctx = grpc.NewContextWithServerTransportStream(ctx, sts)

	body := rpc.GetBody()

	dec := func(msg interface{}) error {
		if body.Data == nil {
			return nil
		}

		if err := h.codec.Unmarshal(body.Data, msg); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		return nil
	}

	resp, appErr := md.Handler(srv.serviceImpl, ctx, dec, h.unaryInterceptor)

	respH := internal.ToKeyValue(sts.GetHeaders())
	respHeader := &rpcheader.RequestHeader{
		Method:  fullMethod,
		Headers: respH,
	}
	respTrailer := &rpcheader.Trailer{
		Metadata: internal.ToKeyValue(sts.GetTrailers()),
	}

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
			Code:    appStatus.Proto().GetCode(),
			Message: appStatus.Proto().GetMessage(),
			Details: appStatus.Proto().GetDetails(),
		}
	}

	var respBody *rpcheader.Body

	if resp != nil {
		data, err := h.codec.Marshal(resp)

		if err == nil {
			respBody = &rpcheader.Body{
				Data: data,
			}
		}
	}

	return &rpcheader.Rpc{
		Id:      rpc.GetId(),
		Header:  respHeader,
		Status:  respStatus,
		Body:    respBody,
		Trailer: respTrailer,
	}
}

func (h *handler) processStreamingRpc(
	srv *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *rpcheader.Rpc,
) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if ch, ok := h.streams[rpc.Id]; ok {
		ch <- rpc
		return nil
	}

	streamId := rpc.Id
	ch := make(chan *rpcheader.Rpc, 1)
	h.streams[streamId] = ch

	go h.newStream(srv, sd, rpc, streamId, ch)
	return nil
}

func (h *handler) newStream(
	srv *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *rpcheader.Rpc,
	streamId uint64,
	rCh chan *rpcheader.Rpc,
) error {
	ctx := context.Background()

	defer h.unregisterStream(streamId)

	r := func(ctx context.Context) (*rpcheader.Rpc, error) {
		select {
		case msg := <-rCh:
			return msg, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	ctx, cancel, err := contextFromHeaders(ctx, rpc.GetHeader())
	if err != nil {
		log.Panicf("failed to processUnaryRpc: %v", err)
	}
	defer cancel()

	stream, err := newServerStream(ctx, streamId, sd.StreamName, r, h.rw.Write)
	if err != nil {
		return err
	}

	info := &grpc.StreamServerInfo{
		FullMethod:     fmt.Sprintf("/%s/%s", srv.name, sd.StreamName),
		IsClientStream: sd.ClientStreams,
		IsServerStream: sd.ServerStreams,
	}
	sts := NewServerTransportStream(info.FullMethod, stream)
	stream.SetContext(grpc.NewContextWithServerTransportStream(ctx, sts))

	if h.streamInterceptor != nil {
		err = h.streamInterceptor(srv.serviceImpl, stream, info, sd.Handler)
	} else {
		err = sd.Handler(srv.serviceImpl, stream)
	}

	err = stream.SendTrailer(err)
	if err != nil {
		log.Printf("ServerStream SendTrailer, %v", err)
		return err
	}

	return nil
}

func (h *handler) unregisterStream(id uint64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if ch, ok := h.streams[id]; ok {
		close(ch)
	}

	delete(h.streams, id)
}

// contextFromHeaders returns a child of the given context that is populated
// using the given headers. The headers are converted to incoming metadata that
// can be retrieved via metadata.FromIncomingContext. If the headers contain a
// gRPC timeout, that is used to create a timeout for the returned context.
func contextFromHeaders(
	parent context.Context,
	h *rpcheader.RequestHeader,
) (context.Context, context.CancelFunc, error) {
	cancel := func() {} // default to no-op
	md, err := internal.ToMetadata(h.Headers)
	if err != nil {
		return parent, cancel, err
	}
	ctx := metadata.NewIncomingContext(parent, md)

	// deadline propagation
	timeout := ""
	for _, rh := range h.Headers {
		if rh.Key == "GRPC-Timeout" {
			timeout = rh.Value
		}
	}
	if timeout != "" {
		// See GRPC wire format, "Timeout" component of request: https://grpc.io/docs/guides/wire.html#requests
		suffix := timeout[len(timeout)-1]
		if timeoutVal, err := strconv.ParseInt(timeout[:len(timeout)-1], 10, 64); err == nil {
			var unit time.Duration
			switch suffix {
			case 'H':
				unit = time.Hour
			case 'M':
				unit = time.Minute
			case 'S':
				unit = time.Second
			case 'm':
				unit = time.Millisecond
			case 'u':
				unit = time.Microsecond
			case 'n':
				unit = time.Nanosecond
			}
			if unit != 0 {
				ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutVal)*unit)
			}
		}
	}
	return ctx, cancel, nil
}
