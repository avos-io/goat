package server

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/internal"
	"github.com/avos-io/goat/types"
)

type serverStream struct {
	ctx context.Context
	id  uint64

	method string
	src    string
	dst    string

	codec encoding.Codec

	rw            types.RpcReadWriter
	statsHandlers []stats.Handler

	protected struct {
		sync.Mutex

		headers      []metadata.MD
		headersSent  bool
		trailers     []metadata.MD
		trailersSent bool
	}
}

var _ grpc.ServerStream = (*serverStream)(nil)

func NewServerStream(
	ctx context.Context,
	id uint64,
	method string,
	src string,
	dst string,
	rw types.RpcReadWriter,
	statsHandlers []stats.Handler,
) (*serverStream, error) {
	return &serverStream{
		ctx:           ctx,
		id:            id,
		method:        method,
		src:           src,
		dst:           dst,
		codec:         encoding.GetCodec(proto.Name),
		rw:            rw,
		statsHandlers: statsHandlers,
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
	ss.protected.Lock()
	defer ss.protected.Unlock()

	if ss.protected.headersSent {
		return fmt.Errorf("headers already sent")
	}

	ss.protected.headers = append(ss.protected.headers, md)

	if !send {
		return nil
	}

	for _, sh := range ss.statsHandlers {
		sh.HandleRPC(ss.ctx, &stats.OutHeader{
			FullMethod: ss.method,
			Header:     metadata.Join(ss.protected.headers...),
		})
	}

	rpc := goatorepo.Rpc{
		Id: ss.id,
		Header: &goatorepo.RequestHeader{
			Method:      ss.method,
			Source:      ss.src,
			Destination: ss.dst,
			Headers:     internal.ToKeyValue(ss.protected.headers...),
		},
	}

	err := ss.rw.Write(ss.ctx, &rpc)
	if err != nil {
		log.Error().Err(err).Msg("ServerStream setHeader: conn.Write")
		return err
	}

	ss.protected.headersSent = true
	return nil
}

// SetTrailer sets the trailer metadata which will be sent with the RPC status.
// When called more than once, all the provided metadata will be merged.
func (ss *serverStream) SetTrailer(md metadata.MD) {
	ss.protected.Lock()
	defer ss.protected.Unlock()

	if ss.protected.trailersSent {
		log.Error().Msg("SetTrailer: trailers already sent!")
		return // I want to scream but I have no mouth
	}

	ss.protected.trailers = append(ss.protected.trailers, md)
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
	ss.protected.Lock()
	defer ss.protected.Unlock()

	body, err := ss.codec.Marshal(m)
	if err != nil {
		log.Error().Err(err).Msg("SendMsg unmarshal")
		return err
	}

	rpc := goatorepo.Rpc{
		Id: ss.id,
		Header: &goatorepo.RequestHeader{
			Method:      ss.method,
			Source:      ss.src,
			Destination: ss.dst,
		},
		Body: &goatorepo.Body{
			Data: body,
		},
	}

	if !ss.protected.headersSent {
		rpc.Header.Headers = internal.ToKeyValue(ss.protected.headers...)
		ss.protected.headersSent = true

		for _, sh := range ss.statsHandlers {
			sh.HandleRPC(ss.ctx, &stats.OutHeader{
				FullMethod: ss.method,
				Header:     metadata.Join(ss.protected.headers...),
			})
		}
	}

	for _, sh := range ss.statsHandlers {
		sh.HandleRPC(ss.ctx, &stats.OutPayload{
			Payload:  m,
			Length:   len(body),
			SentTime: time.Now(),
		})
	}

	err = ss.rw.Write(ss.ctx, &rpc)
	if err != nil {
		log.Error().Err(err).Msg("ServerStream SendMsg: conn.Write")
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
		return errors.Wrap(err, "RecvMsg Read")
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
		err := status.FromProto(&sp).Err()
		return errors.Wrap(err, "RecvMsg")
	}

	err = ss.codec.Unmarshal(rpc.GetBody().GetData(), m)

	if err == nil {
		for _, sh := range ss.statsHandlers {
			sh.HandleRPC(ss.ctx, &stats.InPayload{
				RecvTime: time.Now(),
				Payload:  m,
				Length:   len(rpc.GetBody().GetData()),
			})
		}
	}

	return err
}

// SetContext sets the context for the stream. It is not safe to call SetContext
// after the stream has been used (i.e., SendMsg, SendHeader).
func (ss *serverStream) SetContext(ctx context.Context) {
	ss.ctx = ctx
}

// SendTrailer sends the trailer for the stream, setting the Status to match the
// given error. On completion, the stream is considered closed for writing.
func (ss *serverStream) SendTrailer(trErr error) error {
	ss.protected.Lock()
	defer ss.protected.Unlock()

	if ss.protected.trailersSent {
		return fmt.Errorf("stream closed for writing (wrote trailers)")
	}
	ss.protected.trailersSent = true

	tr := goatorepo.Rpc{
		Id: ss.id,
		Header: &goatorepo.RequestHeader{
			Method:      ss.method,
			Source:      ss.src,
			Destination: ss.dst,
		},
		Status: &goatorepo.ResponseStatus{
			Code:    int32(codes.OK),
			Message: codes.OK.String(),
		},
		Trailer: &goatorepo.Trailer{
			Metadata: internal.ToKeyValue(ss.protected.trailers...),
		},
	}

	if !ss.protected.headersSent {
		tr.Header.Headers = internal.ToKeyValue(ss.protected.headers...)
		ss.protected.headersSent = true
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

	for _, sh := range ss.statsHandlers {
		sh.HandleRPC(ss.ctx, &stats.OutTrailer{
			Trailer: metadata.Join(ss.protected.trailers...),
		})
	}

	err := ss.rw.Write(ss.ctx, &tr)
	if err != nil {
		return err
	}
	return nil
}
