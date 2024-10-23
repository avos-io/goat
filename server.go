package goat

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/internal/server"
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

func StatsHandler(h stats.Handler) ServerOption {
	return serverOptFunc(func(s *Server) {
		s.statsHandlers = append(s.statsHandlers, h)
	})
}

func NewServer(id string, opts ...ServerOption) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	srv := Server{
		id:       id,
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
	id     string

	services map[string]*serviceInfo // service name -> service info

	unaryInterceptor  grpc.UnaryServerInterceptor
	streamInterceptor grpc.StreamServerInterceptor
	statsHandlers     []stats.Handler
}

func (s *Server) Stop() {
	log.Info().Msg("Server stop")
	s.cancel()
}

// grpc.ServiceRegistrar
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	if ss != nil {
		ht := reflect.TypeOf(sd.HandlerType).Elem()
		st := reflect.TypeOf(ss)
		if !st.Implements(ht) {
			log.Panic().Msgf("RegisterService handler of type %v does not satisfy %v", st, ht)
		}
	}

	if _, ok := s.services[sd.ServiceName]; ok {
		log.Panic().Msgf("RegisterService duplicate service registration for %q", sd.ServiceName)
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

func (s *Server) Serve(ctx context.Context, rw RpcReadWriter) error {
	h := newHandler(s.ctx, s, rw)
	err := h.serve(ctx)
	h.cancelAndWaitForStreams()
	return err
}

type unaryRpcArgs struct {
	info *serviceInfo
	md   *grpc.MethodDesc
	rpc  *goatorepo.Rpc
}

type streamHandler struct {
	ch     chan *goatorepo.Rpc
	done   chan struct{}
	cancel context.CancelFunc
}

// handler for a specific goat.RpcReadWriter
type handler struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	srv *Server

	rw    RpcReadWriter
	codec encoding.CodecV2

	mu      sync.Mutex // protects streams
	streams map[uint64]streamHandler

	writeChan    chan *goatorepo.Rpc
	unaryRpcChan chan unaryRpcArgs
}

func newHandler(ctx context.Context, srv *Server, rw RpcReadWriter) *handler {
	ctx, cancel := context.WithCancelCause(ctx)

	return &handler{
		ctx:          ctx,
		cancel:       cancel,
		srv:          srv,
		rw:           rw,
		codec:        encoding.GetCodecV2(proto.Name),
		streams:      map[uint64]streamHandler{},
		writeChan:    make(chan *goatorepo.Rpc),
		unaryRpcChan: make(chan unaryRpcArgs),
	}
}

func (h *handler) serve(clientCtx context.Context) error {
	defer h.cancel(fmt.Errorf("serve done"))

	ctx := h.ctx

	for _, sh := range h.srv.statsHandlers {
		clientCtx = sh.TagConn(clientCtx, &stats.ConnTagInfo{})
		sh.HandleConn(clientCtx, &stats.ConnBegin{})
	}
	defer func() {
		for _, sh := range h.srv.statsHandlers {
			sh.HandleConn(clientCtx, &stats.ConnEnd{})
		}
	}()

	writeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case rpc := <-h.writeChan:
				err := h.rw.Write(writeCtx, rpc)
				if err != nil {
					h.cancel(fmt.Errorf("write error: %v", err))
				}
			case <-writeCtx.Done():
				return
			}
		}
	}()

	unaryRpcCtx, unaryRpcCtxCancel := context.WithCancel(ctx)
	defer unaryRpcCtxCancel()

	const numRpcWorkers = 8

	for i := 0; i < numRpcWorkers; i++ {
		go func() {
			for {
				select {
				case args := <-h.unaryRpcChan:
					h.writeChan <- h.processUnaryRpc(clientCtx, args.info, args.md, args.rpc)
				case <-unaryRpcCtx.Done():
					return
				}
			}
		}()
	}

	for {
		rpc, err := h.rw.Read(ctx)
		if err != nil {
			return errors.Wrap(err, "read error")
		}

		if rpc.GetHeader() == nil {
			log.Warn().Msgf("Server: received RPC without a header: ignoring message")
			continue
		}

		rawMethod := rpc.GetHeader().Method
		service, method, err := parseRawMethod(rawMethod)
		if err != nil {
			log.Warn().Msgf("Server: received RPC without invalid header, failed to parse %s: ignoring message", rawMethod)
			continue
		}

		if rpc.GetHeader().Destination != h.srv.id {
			log.Warn().
				Msgf("Server %s: invalid destination %s: ignoring message", h.srv.id, rpc.GetHeader().Destination)
			continue
		}

		si, known := h.srv.services[service]
		if !known {
			log.Warn().Msgf("Server: unknown service, %s: ignoring message", service)
			continue
		}
		if md, ok := si.methods[method]; ok {
			select {
			case h.unaryRpcChan <- unaryRpcArgs{si, md, rpc}:
			case <-h.ctx.Done():
				return h.ctx.Err()
			}
			continue
		}
		if sd, ok := si.streams[method]; ok {
			if err := h.processStreamingRpc(clientCtx, si, sd, rpc); err != nil {
				return err
			}
			continue
		}
		log.Warn().Msgf("Server: unhandled method, %s %s: ignoring message", service, method)
	}
}

func (h *handler) cancelAndWaitForStreams() {
	h.mu.Lock()
	for len(h.streams) > 0 {
		// Just get the first streamHandler we find in the map and wait on it - order
		// doesn't matter here, we just want to grab one.
		var sh streamHandler
		for _, sh = range h.streams {
			break
		}
		h.mu.Unlock()

		sh.cancel()
		<-sh.done

		h.mu.Lock()
	}
	h.mu.Unlock()
}

func (h *handler) processUnaryRpc(
	clientCtx context.Context,
	info *serviceInfo,
	md *grpc.MethodDesc,
	rpc *goatorepo.Rpc,
) *goatorepo.Rpc {
	ctx, cancel, err := contextFromHeaders(clientCtx, rpc.GetHeader())
	if err != nil {
		log.Panic().Err(err).Msg("Server: failed to get context from headers")
	}
	defer cancel()

	var appErr error
	fullMethod := fmt.Sprintf("/%s/%s", info.name, md.MethodName)

	beginTime := time.Now()
	ctx = internal.StatsStartServerRPC(h.srv.statsHandlers, false, beginTime, fullMethod, false, false, ctx)
	defer func() {
		internal.StatsEndRPC(h.srv.statsHandlers, false, beginTime, appErr, ctx)
	}()

	sts := server.NewUnaryServerTransportStream(fullMethod)
	ctx = grpc.NewContextWithServerTransportStream(ctx, sts)

	body := rpc.GetBody()

	dec := func(msg interface{}) error {
		if body.GetData() == nil {
			return nil
		}

		buf := mem.NewBuffer(&body.Data, nil)
		bs := mem.BufferSlice{buf}

		if err := h.codec.Unmarshal(bs, msg); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		for _, sh := range h.srv.statsHandlers {
			sh.HandleRPC(ctx, &stats.InPayload{
				RecvTime: time.Now(),
				Payload:  msg,
				Length:   len(body.GetData()),
			})
		}
		return nil
	}

	var resp any
	resp, appErr = md.Handler(info.serviceImpl, ctx, dec, h.srv.unaryInterceptor)

	respH := internal.ToKeyValue(sts.GetHeaders())
	respHeader := &goatorepo.RequestHeader{
		Method:      fullMethod,
		Headers:     respH,
		Source:      rpc.Header.Destination,
		Destination: rpc.Header.Source,
	}
	if len(rpc.Header.ProxyRecord) > 1 {
		respHeader.ProxyNext = rpc.Header.ProxyRecord[0 : len(rpc.Header.ProxyRecord)-1]
	}
	respTrailer := &goatorepo.Trailer{
		Metadata: internal.ToKeyValue(sts.GetTrailers()),
	}

	var respStatus *goatorepo.ResponseStatus

	if appErr != nil {
		st, ok := status.FromError(appErr)
		if !ok {
			st = status.FromContextError(appErr)
		}
		respStatus = &goatorepo.ResponseStatus{
			Code:    st.Proto().GetCode(),
			Message: st.Proto().GetMessage(),
			Details: st.Proto().GetDetails(),
		}
	}

	var respBody *goatorepo.Body
	var data mem.BufferSlice

	if resp != nil {
		data, err = h.codec.Marshal(resp)

		if err == nil {
			respBody = &goatorepo.Body{
				Data: data.Materialize(),
			}
		}
	}

	for _, sh := range h.srv.statsHandlers {
		sh.HandleRPC(ctx, &stats.OutHeader{
			FullMethod: fullMethod,
			Header:     sts.GetHeaders(),
		})
		sh.HandleRPC(ctx, &stats.OutPayload{
			Payload:  resp,
			Length:   len(data),
			SentTime: time.Now(),
		})
		sh.HandleRPC(ctx, &stats.OutTrailer{
			Trailer: sts.GetTrailers(),
		})
	}

	return &goatorepo.Rpc{
		Id:      rpc.GetId(),
		Header:  respHeader,
		Status:  respStatus,
		Body:    respBody,
		Trailer: respTrailer,
	}
}

func (h *handler) processStreamingRpc(
	clientCtx context.Context,
	info *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *goatorepo.Rpc,
) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	resetStream := rpc.GetReset_() != nil && rpc.GetReset_().Type == "RST_STREAM"

	if handler, ok := h.streams[rpc.Id]; ok {
		if resetStream {
			handler.cancel()
		} else {
			select {
			case handler.ch <- rpc:
			case <-clientCtx.Done():
				return clientCtx.Err()
			case <-h.ctx.Done():
				return context.Cause(h.ctx)
			}
		}
		return nil
	}

	if resetStream {
		// In this case we've got a reset for something that no longer exists, so can
		// safely ignore it.
		return nil
	}

	if rpc.GetBody() != nil {
		// The first Rpc in a stream is used to tell the server to start the stream;
		// it must have an empty body. If this isn't the case, it must be because
		// we've missed the first Rpc in the stream.
		log.Info().Msgf("did not expect body: calling RST stream %d", rpc.Id)
		return h.resetStream(rpc)
	}

	if rpc.GetTrailer() != nil {
		// The client may send a trailer to end a stream after we've already ended
		// it, in which case we don't want to lazily create a new stream here.
		return nil
	}

	ctx, cancel, err := contextFromHeaders(clientCtx, rpc.GetHeader())
	if err != nil {
		log.Info().Msgf("invalid headers: calling RST stream %d", rpc.Id)
		return h.resetStream(rpc)
	}

	streamId := rpc.Id

	h.streams[streamId] = streamHandler{
		ch:     make(chan *goatorepo.Rpc, 1),
		done:   make(chan struct{}, 1),
		cancel: cancel,
	}

	go h.runStream(info, sd, rpc, streamId, ctx, h.streams[streamId])
	return nil
}

func (h *handler) runStream(
	info *serviceInfo,
	sd *grpc.StreamDesc,
	rpc *goatorepo.Rpc,
	streamId uint64,
	ctx context.Context,
	handler streamHandler,
) error {
	defer h.unregisterStream(streamId)

	readerFunc := func(ctx context.Context) (*goatorepo.Rpc, error) {
		select {
		case msg, ok := <-handler.ch:
			if !ok {
				return nil, fmt.Errorf("rCh closed")
			}
			return msg, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	writerFunc := func(ctx context.Context, r *goatorepo.Rpc) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case h.writeChan <- r:
			break
		}
		return nil
	}
	rw := internal.NewFnReadWriter(readerFunc, writerFunc)

	defer handler.cancel()

	var appErr error
	si := &grpc.StreamServerInfo{
		FullMethod:     fmt.Sprintf("/%s/%s", info.name, sd.StreamName),
		IsClientStream: sd.ClientStreams,
		IsServerStream: sd.ServerStreams,
	}

	beginTime := time.Now()
	ctx = internal.StatsStartServerRPC(h.srv.statsHandlers, false, beginTime, si.FullMethod, si.IsClientStream, si.IsServerStream, ctx)
	defer func() {
		internal.StatsEndRPC(h.srv.statsHandlers, false, beginTime, appErr, ctx)
	}()

	stream, err := server.NewServerStream(
		ctx,
		streamId,
		si.FullMethod,
		rpc.Header.Destination,
		rpc.Header.Source,
		rw,
		h.srv.statsHandlers,
	)
	if err != nil {
		log.Error().Err(err).Msg("Server: newServerStream failed")
		return err
	}

	sts := server.NewServerTransportStream(si.FullMethod, stream)
	stream.SetContext(grpc.NewContextWithServerTransportStream(ctx, sts))

	if h.srv.streamInterceptor != nil {
		appErr = h.srv.streamInterceptor(info.serviceImpl, stream, si, sd.Handler)
	} else {
		appErr = sd.Handler(info.serviceImpl, stream)
	}

	err = stream.SendTrailer(appErr)
	if err != nil {
		return err
	}

	return nil
}

func (h *handler) unregisterStream(id uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if handler, ok := h.streams[id]; ok {
		handler.cancel()
		handler.done <- struct{}{}
	}

	delete(h.streams, id)
}

// resetStream instructs the caller to tear down and restart the stream. We call
// this if something has gone irrecoverably wrong in the stream.
func (h *handler) resetStream(rpc *goatorepo.Rpc) error {
	reset := &goatorepo.Rpc{
		Id: rpc.GetId(),
		Header: &goatorepo.RequestHeader{
			Method:      rpc.GetHeader().GetMethod(),
			Source:      rpc.GetHeader().GetDestination(),
			Destination: rpc.GetHeader().GetSource(),
		},
		Reset_: &goatorepo.Reset{
			Type: "RST_STREAM",
		},
		Trailer: &goatorepo.Trailer{},
	}

	if len(rpc.Header.ProxyRecord) > 1 {
		reset.Header.ProxyNext = rpc.Header.ProxyRecord[0 : len(rpc.Header.ProxyRecord)-1]
	}

	// XXX: error handling?
	return h.rw.Write(h.ctx, reset)
}

// contextFromHeaders returns a new incoming context with metadata populated
// by the given request headers. If the given headers contain a GRPC-Timeout, it
// is used to set the deadline on the returned context.
func contextFromHeaders(
	parent context.Context,
	h *goatorepo.RequestHeader,
) (context.Context, context.CancelFunc, error) {
	md, err := internal.ToMetadata(h.Headers)
	if err != nil {
		ctx, cancel := context.WithCancel(parent)
		return ctx, cancel, err
	}
	ctx := metadata.NewIncomingContext(parent, md)

	for _, hdr := range h.Headers {
		if strings.ToLower(hdr.Key) == "grpc-timeout" {
			if timeout, ok := parseGrpcTimeout(hdr.Value); ok {
				ctx, cancel := context.WithTimeout(ctx, timeout)
				return ctx, cancel, nil
			}
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	return ctx, cancel, nil
}

// See https://grpc.io/docs/guides/wire.html#requests
func parseGrpcTimeout(timeout string) (time.Duration, bool) {
	if timeout == "" {
		return 0, false
	}
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
