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

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/avos-io/goat"
	wrapped "github.com/avos-io/goat/gen"
	"github.com/avos-io/goat/internal"
)

// ServerOption is an option used when constructing a NewServer.
type ServerOption interface {
	apply(*Server)
}

type serverOptFunc func(*Server)

func (fn serverOptFunc) apply(s *Server) {
	fn(s)
}

// UnaryInterceptor returns a ServerOption that sets the UnaryServerInterceptor
// for the server. Only one unary interceptor can be installed. The construction
// of multiple interceptors (e.g., chaining) can be implemented at the caller.
func UnaryInterceptor(i grpc.UnaryServerInterceptor) ServerOption {
	return serverOptFunc(func(s *Server) {
		s.unaryInterceptor = i
	})
}

// StreamInterceptor returns a ServerOption that sets the StreamServerInterceptor
// for the server. Only one stream interceptor can be installed.
func StreamInterceptor(i grpc.StreamServerInterceptor) ServerOption {
	return serverOptFunc(func(s *Server) {
		s.streamInterceptor = i
	})
}

func NewServer(opts ...ServerOption) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	srv := Server{
		ctx:      ctx,
		cancel:   cancel,
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

type Server struct {
	ctx    context.Context
	cancel context.CancelFunc

	services map[string]*serviceInfo // service name -> service info

	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor
}

func (s *Server) Stop() {
	log.Printf("Server stop")
	s.cancel()
}

// grpc.ServiceRegistrar
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			log.Fatalf("RegisterService handler of type %v does not satisfy %v", st, ht)
		}
	}

	if _, ok := s.services[sd.ServiceName]; ok {
		log.Fatalf("RegisterService duplicate service registration for %q", sd.ServiceName)
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

func (s *Server) Serve(rw goat.RpcReadWriter) error {
	h := newHandler(s.ctx, s, rw)
	h.serve()
	return nil
}

// handler for a specific goat.RpcReadWriter
type handler struct {
	ctx context.Context

	srv *Server

	rw    goat.RpcReadWriter
	codec encoding.Codec

	mu      sync.Mutex // protects streams
	streams map[uint64]chan *wrapped.Rpc
}

func newHandler(ctx context.Context, srv *Server, rw goat.RpcReadWriter) *handler {
	return &handler{
		ctx:     ctx,
		srv:     srv,
		rw:      rw,
		codec:   encoding.GetCodec(proto.Name),
		streams: map[uint64]chan *wrapped.Rpc{},
	}
}

func (h *handler) serve() error {
	for {
		rpc, err := h.rw.Read(h.ctx)
		if err != nil {
			log.Printf("read err: %v", err)
			return err
		}

		if rpc.GetHeader() == nil {
			return fmt.Errorf("no header")
		}

		rawMethod := rpc.GetHeader().Method
		service, method, err := parseRawMethod(rawMethod)
		if err != nil {
			log.Printf("failed to parse %s", rawMethod)
			return err
		}

		si, known := h.srv.services[service]
		if !known {
			log.Printf("unknown service, %s", service)
			continue
		}
		if md, ok := si.methods[method]; ok {
			resp := h.processUnaryRpc(si, md, rpc)
			h.rw.Write(h.ctx, resp)
			continue
		}
		if sd, ok := si.streams[method]; ok {
			h.processStreamingRpc(si, sd, rpc)
			continue
		}
		log.Printf("unhandled method, %s %s", service, method)
	}
}

func (h *handler) processUnaryRpc(
	info *serviceInfo,
	md *grpc.MethodDesc,
	rpc *wrapped.Rpc,
) *wrapped.Rpc {
	ctx := context.Background()

	ctx, cancel, err := contextFromHeaders(ctx, rpc.GetHeader())
	if err != nil {
		log.Panicf("failed to processUnaryRpc: %v", err)
	}
	defer cancel()

	fullMethod := fmt.Sprintf("%s/%s", info.name, md.MethodName)
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

	resp, appErr := md.Handler(info.serviceImpl, ctx, dec, h.srv.unaryInterceptor)

	respH := internal.ToKeyValue(sts.GetHeaders())
	respHeader := &wrapped.RequestHeader{
		Method:  fullMethod,
		Headers: respH,
	}
	respTrailer := &wrapped.Trailer{
		Metadata: internal.ToKeyValue(sts.GetTrailers()),
	}

	var respStatus *wrapped.ResponseStatus

	if appErr != nil {
		st, ok := status.FromError(appErr)
		if !ok {
			st = status.FromContextError(appErr)
		}
		respStatus = &wrapped.ResponseStatus{
			Code:    st.Proto().GetCode(),
			Message: st.Proto().GetMessage(),
			Details: st.Proto().GetDetails(),
		}
	}

	var respBody *wrapped.Body

	if resp != nil {
		data, err := h.codec.Marshal(resp)

		if err == nil {
			respBody = &wrapped.Body{
				Data: data,
			}
		}
	}

	return &wrapped.Rpc{
		Id:      rpc.GetId(),
		Header:  respHeader,
		Status:  respStatus,
		Body:    respBody,
		Trailer: respTrailer,
	}
}

func (h *handler) processStreamingRpc(
	info *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *wrapped.Rpc,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.streams[rpc.Id]; ok {
		ch <- rpc
		return nil
	}

	streamId := rpc.Id
	ch := make(chan *wrapped.Rpc, 1)
	h.streams[streamId] = ch

	go h.runStream(info, sd, rpc, streamId, ch)
	return nil
}

func (h *handler) runStream(
	info *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *wrapped.Rpc,
	streamId uint64,
	rCh chan *wrapped.Rpc,
) error {
	ctx := context.Background()

	defer h.unregisterStream(streamId)

	if rpc.GetTrailer() != nil {
		// The client may send a trailer to end a stream after we've already ended
		// it, in which case we don't want to lazily create a new stream here.
		log.Printf("ignoring client EOF for torn-down stream %d", streamId)
		return nil
	}

	r := func(ctx context.Context) (*wrapped.Rpc, error) {
		select {
		case msg, ok := <-rCh:
			if !ok {
				return nil, fmt.Errorf("rCh closed")
			}
			return msg, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	rw := goat.NewFnReadWriter(r, h.rw.Write)

	ctx, cancel, err := contextFromHeaders(ctx, rpc.GetHeader())
	if err != nil {
		log.Panicf("failed to create newStream: %v", err)
	}
	defer cancel()

	si := &grpc.StreamServerInfo{
		FullMethod:     fmt.Sprintf("%s/%s", info.name, sd.StreamName),
		IsClientStream: sd.ClientStreams,
		IsServerStream: sd.ServerStreams,
	}

	stream, err := newServerStream(ctx, streamId, si.FullMethod, rw)
	if err != nil {
		return err
	}

	sts := NewServerTransportStream(si.FullMethod, stream)
	stream.SetContext(grpc.NewContextWithServerTransportStream(ctx, sts))

	var appErr error
	if h.srv.streamInterceptor != nil {
		appErr = h.srv.streamInterceptor(info.serviceImpl, stream, si, sd.Handler)
	} else {
		appErr = sd.Handler(info.serviceImpl, stream)
	}

	err = stream.SendTrailer(appErr)
	if err != nil {
		log.Printf("runStream trailer err, %v", err)
		return err
	}

	return nil
}

func (h *handler) unregisterStream(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.streams[id]; ok {
		close(ch)
	}

	delete(h.streams, id)
}

// contextFromHeaders returns a new incoming context with metadata populated
// by the given request headers. If the given headers contain a GRPC-Timeout, it
// is used to set the deadline on the returned context.
func contextFromHeaders(
	parent context.Context,
	h *wrapped.RequestHeader,
) (context.Context, context.CancelFunc, error) {
	cancel := func() {}
	md, err := internal.ToMetadata(h.Headers)
	if err != nil {
		return parent, cancel, err
	}
	ctx := metadata.NewIncomingContext(parent, md)

	for _, hdr := range h.Headers {
		if hdr.Key == "GRPC-Timeout" {
			if timeout, ok := parseGrpcTimeout(hdr.Value); ok {
				ctx, cancel = context.WithTimeout(ctx, timeout)
				break
			}
		}
	}
	return ctx, cancel, nil
}

// See https://grpc.io/docs/guides/wire.html#requests
func parseGrpcTimeout(timeout string) (time.Duration, bool) {
	suffix := timeout[len(timeout)-1]

	val, err := strconv.ParseInt(timeout[:len(timeout)-1], 10, 64)
	if err != nil {
		return 0, false
	}
	getUnit := func(suffix byte) time.Duration {
		switch suffix {
		case 'H':
			return time.Hour
		case 'M':
			return time.Minute
		case 'S':
			return time.Second
		case 'm':
			return time.Millisecond
		case 'u':
			return time.Microsecond
		case 'n':
			return time.Nanosecond
		default:
			return 0
		}
	}
	unit := getUnit(suffix)
	if unit == 0 {
		return 0, false
	}

	return time.Duration(val) * unit, true
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
