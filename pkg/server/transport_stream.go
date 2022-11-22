package server

import (
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// UnaryServerTransportStream is a minimal grpc.ServerTransportStream.
//
// The functionality for setting headers and trailers in gRPC requires a stream
// to be in context, so this stream collects headers and trailers on behalf of
// a unary handler, allowing them to be later retrieved by the handler.
//
// See grpc.NewContextWithServerTransportStream.
type UnaryServerTransportStream interface {
	grpc.ServerTransportStream
	GetHeaders() metadata.MD
	GetTrailers() metadata.MD
}

func NewUnaryServerTransportStream(name string) UnaryServerTransportStream {
	return &unaryServerTransportStream{FullMethod: name}
}

type unaryServerTransportStream struct {
	FullMethod string

	mutex       sync.Mutex
	headers     metadata.MD
	headersSent bool
	trailers    metadata.MD
}

// Method returns the full method (serviceName + method) of the call.
func (sts *unaryServerTransportStream) Method() string {
	return sts.FullMethod
}

// SetHeader adds the given metadata to the set of response headers which will
// be sent to the client.
func (sts *unaryServerTransportStream) SetHeader(md metadata.MD) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()
	return sts.setHeaderLocked(md)
}

// SendHeader adds the given metadata to the set of response headers which will
// be sent to the client. SendHeader doesn't actually send the header, but marks
// the headers as sent, such that any further calls to SetHeader or SendHeader
// will fail.
func (sts *unaryServerTransportStream) SendHeader(md metadata.MD) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()
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
	sts.mutex.Lock()
	defer sts.mutex.Unlock()
	return sts.headers
}

// SetTrailer satisfies the grpc.ServerTransportStream, adding the given
// metadata to the set of response trailers that will be sent to the client.
func (sts *unaryServerTransportStream) SetTrailer(md metadata.MD) error {
	sts.mutex.Lock()
	defer sts.mutex.Unlock()

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
	sts.mutex.Lock()
	defer sts.mutex.Unlock()
	return sts.trailers
}

// serverTransportStream implements grpc.serverTransportStream, mostly by
// wrapping a grpc.Server stream and delegating calls to it.
type serverTransportStream struct {
	FullMethod string
	Stream     grpc.ServerStream
}

func NewServerTransportStream(
	fullMethod string,
	stream grpc.ServerStream,
) grpc.ServerTransportStream {
	return &serverTransportStream{FullMethod: fullMethod, Stream: stream}
}

// Method returns the full method (serviceName + stream method) of the stream.
func (sts *serverTransportStream) Method() string {
	return sts.FullMethod
}

// SetHeader sets the header on the underlying stream.
func (sts *serverTransportStream) SetHeader(md metadata.MD) error {
	return sts.Stream.SetHeader(md)
}

// SendHeader sends the header on the underlying stream.
func (sts *serverTransportStream) SendHeader(md metadata.MD) error {
	return sts.Stream.SendHeader(md)
}

// SetTrailer sets the trailer on the underlying stream.
func (sts *serverTransportStream) SetTrailer(md metadata.MD) error {
	sts.Stream.SetTrailer(md)
	return nil
}
