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

// WithUnaryInterceptor returns a DialOption that specifies the interceptor for
// unary RPCs.
//
// WARNING: the interceptor will be called with a nil *grpc.ClientConn. The
// google.golang.org/grpc interface here is a little leaky and exposes its
// particular ClientConn. If we wanted, we could implement our own interceptor
// interface, but sticking with this one for now opens up the possibility of
// using e.g., google.golang.org/grpc/metadata as it is, rather than re-
// implementing that functionality ourselves.
func WithUnaryInterceptor(i grpc.UnaryClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.unaryInterceptor = i
	})
}

// WithStreamInterceptor returns a DialOption that specifies the interceptor for
// streaming RPCs.
//
// WARNING: the interceptor will be called with a nil *grpc.ClientConn. The
// google.golang.org/grpc interface here is a little leaky and exposes its
// particular ClientConn. If we wanted, we could implement our own interceptor
// interface, but sticking with this one for now opens up the possibility of
// using e.g., google.golang.org/grpc/metadata as it is, rather than re-
// implementing that functionality ourselves.
func WithStreamInterceptor(i grpc.StreamClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.streamInterceptor = i
	})
}

// WithChainUnaryInterceptor returns a DialOption that specifies the chained
// interceptor for unary RPCs. The first interceptor will be the outer most,
// while the last interceptor will be the inner most wrapper around the real call.
// All interceptors added by this method will be chained, and the interceptor
// defined by WithUnaryInterceptor will always be prepended to the chain.
//
// WARNING: the interceptors will be called with a nil *grpc.ClientConn. The
// google.golang.org/grpc interface here is a little leaky and exposes its
// particular ClientConn. If we wanted, we could implement our own interceptor
// interface, but sticking with this one for now opens up the possibility of
// using e.g., google.golang.org/grpc/metadata as it is, rather than re-
// implementing that functionality ourselves.
func WithChainUnaryInterceptor(is ...grpc.UnaryClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.chainUnaryInterceptors = append(cc.chainUnaryInterceptors, is...)
	})
}

// WithChainStreamInterceptor returns a DialOption that specifies the chained
// interceptor for streaming RPCs. The first interceptor will be the outer most,
// while the last interceptor will be the inner most wrapper around the real call.
// All interceptors added by this method will be chained, and the interceptor
// defined by WithStreamInterceptor will always be prepended to the chain.
//
// WARNING: the interceptors will be called with a nil *grpc.ClientConn. The
// google.golang.org/grpc interface here is a little leaky and exposes its
// particular ClientConn. If we wanted, we could implement our own interceptor
// interface, but sticking with this one for now opens up the possibility of
// using e.g., google.golang.org/grpc/metadata as it is, rather than re-
// implementing that functionality ourselves.
func WithChainStreamInterceptor(is ...grpc.StreamClientInterceptor) DialOption {
	return dialOptFunc(func(cc *ClientConn) {
		cc.chainStreamInterceptors = append(cc.chainStreamInterceptors, is...)
	})
}
