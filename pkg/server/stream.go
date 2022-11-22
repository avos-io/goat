package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	spb "google.golang.org/genproto/googleapis/rpc/status"
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

type ServerStream interface {
	grpc.ServerStream
	SetContext(context.Context)
	SendTrailer(error) error
}

type serverStream struct {
	ctx    context.Context
	id     uint64
	method string

	codec encoding.Codec

	rw goat.RpcReadWriter

	mu           sync.Mutex // protects headers, headersSent, trailers, trailersSent
	headers      []metadata.MD
	headersSent  bool
	trailers     []metadata.MD
	trailersSent bool
}

func newServerStream(
	ctx context.Context,
	id uint64,
	method string,
	rw goat.RpcReadWriter,
) (ServerStream, error) {
	return &serverStream{
		ctx:    ctx,
		id:     id,
		method: method,
		codec:  encoding.GetCodec(proto.Name),
		rw:     rw,
	}, nil
}

// SetHeader sets the header metadata. It may be called multiple times.
// When call multiple times, all the provided metadata will be merged.
// All the metadata will be sent out when one of the following happens:
//   - ServerStream.SendHeader() is called;
//   - The first response is sent out;
//   - An RPC status is sent out (error or success).
func (ss *serverStream) SetHeader(md metadata.MD) error {
	return ss.setHeader(md, false)
}

// SendHeader sends the header metadata.
// The provided md and headers set by SetHeader() will be sent.
// It fails if called multiple times.
func (ss *serverStream) SendHeader(md metadata.MD) error {
	return ss.setHeader(md, true)
}

func (ss *serverStream) setHeader(md metadata.MD, send bool) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.headersSent {
		return fmt.Errorf("headers already sent")
	}

	ss.headers = append(ss.headers, md)

	if !send {
		return nil
	}

	rpc := wrapped.Rpc{
		Id: ss.id,
		Header: &wrapped.RequestHeader{
			Method:  ss.method,
			Headers: internal.ToKeyValue(ss.headers...),
		},
	}

	err := ss.rw.Write(ss.ctx, &rpc)
	if err != nil {
		log.Printf("ServerStream setHeader: conn.Write %v", err)
		return err
	}

	ss.headersSent = true
	return nil
}

// SetTrailer sets the trailer metadata which will be sent with the RPC status.
// When called more than once, all the provided metadata will be merged.
func (ss *serverStream) SetTrailer(md metadata.MD) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.trailersSent {
		log.Printf("SetTrailer: trailers already sent!")
		return // I want to scream but I have no mouth
	}

	ss.trailers = append(ss.trailers, md)
}

// Context returns the context for this stream.
func (ss *serverStream) Context() context.Context {
	return ss.ctx
}

// SendMsg sends a message. On error, SendMsg aborts the stream and the
// error is returned directly.
//
// SendMsg blocks until:
//   - There is sufficient flow control to schedule m with the transport, or
//   - The stream is done, or
//   - The stream breaks.
//
// SendMsg does not wait until the message is received by the client. An
// untimely stream closure may result in lost messages.
//
// It is safe to have a goroutine calling SendMsg and another goroutine
// calling RecvMsg on the same stream at the same time, but it is not safe
// to call SendMsg on the same stream in different goroutines.
func (ss *serverStream) SendMsg(m interface{}) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	body, err := ss.codec.Marshal(m)
	if err != nil {
		return err
	}

	rpc := wrapped.Rpc{
		Id: ss.id,
		Header: &wrapped.RequestHeader{
			Method: ss.method,
		},
		Body: &wrapped.Body{
			Data: body,
		},
	}

	if !ss.headersSent {
		rpc.Header.Headers = internal.ToKeyValue(ss.headers...)
		ss.headersSent = true
	}

	err = ss.rw.Write(ss.ctx, &rpc)
	if err != nil {
		log.Printf("ServerStream SendMsg: conn.Write %v", err)
		return err
	}

	return nil
}

// RecvMsg blocks until it receives a message into m or the stream is
// done. It returns io.EOF when the client has performed a CloseSend. On
// any non-EOF error, the stream is aborted and the error contains the
// RPC status.
//
// It is safe to have a goroutine calling SendMsg and another goroutine
// calling RecvMsg on the same stream at the same time, but it is not
// safe to call RecvMsg on the same stream in different goroutines.
func (ss *serverStream) RecvMsg(m interface{}) error {
	rpc, err := ss.rw.Read(ss.ctx)
	if err != nil {
		return err
	}

	if rpc.GetTrailer() != nil {
		st := rpc.GetStatus()
		if st.GetCode() == int32(codes.OK) {
			return io.EOF
		}
		sp := spb.Status{
			Code:    st.GetCode(),
			Message: st.GetMessage(),
			Details: st.GetDetails(),
		}
		return status.FromProto(&sp).Err()
	}

	return ss.codec.Unmarshal(rpc.GetBody().GetData(), m)
}

// SetContext sets the context for the stream. It is not safe to call SetContext
// after the stream has been used (i.e., SendMsg, SendHeader).
func (ss *serverStream) SetContext(ctx context.Context) {
	ss.ctx = ctx
}

// SendTrailer sends the trailer for the stream, setting the Status to match the
// given error. On completion, the stream is considered closed for writing.
func (ss *serverStream) SendTrailer(trErr error) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	if ss.trailersSent {
		return fmt.Errorf("stream closed for writing (wrote trailers)")
	}
	ss.trailersSent = true

	tr := wrapped.Rpc{
		Id: ss.id,
		Header: &wrapped.RequestHeader{
			Method: ss.method,
		},
		Status: &wrapped.ResponseStatus{
			Code:    int32(codes.OK),
			Message: codes.OK.String(),
		},
		Trailer: &wrapped.Trailer{
			Metadata: internal.ToKeyValue(ss.trailers...),
		},
	}

	if !ss.headersSent {
		tr.Header.Headers = internal.ToKeyValue(ss.headers...)
		ss.headersSent = true
	}

	if trErr != nil {
		st, _ := status.FromError(trErr)
		if st.Code() == codes.OK {
			// We know an error *did* occur, so re-write (only) the code
			stpb := st.Proto()
			stpb.Code = int32(codes.Internal)
			st = status.FromProto(stpb)
		}
		sp := st.Proto()
		tr.Status.Code = sp.GetCode()
		tr.Status.Message = sp.GetMessage()
		tr.Status.Details = sp.GetDetails()
	}

	err := ss.rw.Write(ss.ctx, &tr)
	if err != nil {
		log.Printf("ServerStream SendTrailer: conn.Write %v", err)
		return err
	}
	return nil
}
