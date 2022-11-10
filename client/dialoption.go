package client

import "google.golang.org/grpc"

// DialOption is an option used when constructing a NewWebsocketClientConn.
type DialOption interface {
	apply(*ClientConn)
}

type dialOptFunc func(*ClientConn)

func (fn dialOptFunc) apply(s *ClientConn) {
	fn(s)
}

// WARNING: the interceptors here will be called with a nil *grpc.ClientConn.
// The google.golang.org/grpc interface here is a little leaky and exposes its
// particular ClientConn. If we wanted, we could implement our own interceptor
// interface, but sticking with this one for now opens up the possibility of
// using e.g., google.golang.org/grpc/metadata as it is, rather than re-
// implementing that functionality ourselves.

// WithUnaryInterceptor returns a DialOption that specifies the interceptor for
// unary RPCs.
//
// WARNING: the interceptor will receive a nil *grpc.ClientConn. See pkg doc.
func WithUnaryInterceptor(i grpc.UnaryClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.unaryInterceptor = i
	})
}

// WithStreamInterceptor returns a DialOption that specifies the interceptor for
// streaming RPCs.
//
// WARNING: the interceptor will receive a nil *grpc.ClientConn. See pkg doc.
func WithStreamInterceptor(i grpc.StreamClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.streamInterceptor = i
	})
}
