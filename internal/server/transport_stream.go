package server

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// unaryServerTransportStream is a minimal grpc.ServerTransportStream.
//
// The functionality for setting headers and trailers in gRPC requires a stream
// to be in context, so this stream collects headers and trailers on behalf of
// a unary handler, allowing them to be later retrieved by the handler.
//
// See grpc.NewContextWithServerTransportStream.
type unaryServerTransportStream struct {
	fullMethod string

	mu          sync.Mutex
	headers     metadata.MD
	headersSent bool
	trailers    metadata.MD
}

var _ grpc.ServerTransportStream = (*unaryServerTransportStream)(nil)

func NewUnaryServerTransportStream(name string) *unaryServerTransportStream {
	return &unaryServerTransportStream{fullMethod: name}
}

// Method returns the full method (serviceName + method) of the call.
func (sts *unaryServerTransportStream) Method() string {
	return sts.fullMethod
}

// SetHeader adds the given metadata to the set of response headers which will
// be sent to the client.
func (sts *unaryServerTransportStream) SetHeader(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.setHeaderLocked(md)
}

// SendHeader adds the given metadata to the set of response headers which will
// be sent to the client. SendHeader doesn't actually send the header, but marks
// the headers as sent, such that any further calls to SetHeader or SendHeader
// will fail.
func (sts *unaryServerTransportStream) SendHeader(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	if err := sts.setHeaderLocked(md); err != nil {
		return err
	}
	sts.headersSent = true
	return nil
}

func (sts *unaryServerTransportStream) setHeaderLocked(md metadata.MD) error {
	if sts.headersSent {
		return fmt.Errorf("headers already sent")
	}
	if sts.headers == nil {
		sts.headers = metadata.MD{}
	}
	sts.headers = metadata.Join(sts.headers, md)
	return nil
}

// GetHeaders returns the cumulative set of headers set by calls to SetHeader
// and SendHeader. This is used by a server to gather the headers that must
// actually be sent to a client.
func (sts *unaryServerTransportStream) GetHeaders() metadata.MD {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.headers
}

// SetTrailer satisfies the grpc.ServerTransportStream, adding the given
// metadata to the set of response trailers that will be sent to the client.
func (sts *unaryServerTransportStream) SetTrailer(md metadata.MD) error {
	sts.mu.Lock()
	defer sts.mu.Unlock()

	if sts.trailers == nil {
		sts.trailers = metadata.MD{}
	}
	sts.trailers = metadata.Join(sts.trailers, md)
	return nil
}

// GetTrailers returns the cumulative set of trailers set by calls to
// SetTrailer. This is used by a server to gather the headers that must actually
// be sent to a client.
func (sts *unaryServerTransportStream) GetTrailers() metadata.MD {
	sts.mu.Lock()
	defer sts.mu.Unlock()
	return sts.trailers
}

// serverTransportStream implements grpc.serverTransportStream, mostly by
// delegating calls to the wrapped grpc.ServerStream.
type serverTransportStream struct {
	fullMethod string
	stream     grpc.ServerStream
}

var _ grpc.ServerTransportStream = (*serverTransportStream)(nil)

func NewServerTransportStream(
	fullMethod string,
	stream grpc.ServerStream,
) *serverTransportStream {
	return &serverTransportStream{fullMethod: fullMethod, stream: stream}
}

// Method returns the full method (serviceName + stream method) of the stream.
func (sts *serverTransportStream) Method() string {
	return sts.fullMethod
}

// SetHeader sets the header on the underlying stream.
func (sts *serverTransportStream) SetHeader(md metadata.MD) error {
	return sts.stream.SetHeader(md)
}

// SendHeader sends the header on the underlying stream.
func (sts *serverTransportStream) SendHeader(md metadata.MD) error {
	return sts.stream.SendHeader(md)
}

// SetTrailer sets the trailer on the underlying stream.
func (sts *serverTransportStream) SetTrailer(md metadata.MD) error {
	sts.stream.SetTrailer(md)
	return nil
}
