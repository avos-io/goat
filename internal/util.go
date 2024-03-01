package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"

	goatorepo "github.com/avos-io/goat/gen/goatorepo"
	"github.com/avos-io/goat/types"
)

func ToKeyValue(mds ...metadata.MD) []*goatorepo.KeyValue {
	h := []*goatorepo.KeyValue{}
	for k, vs := range metadata.Join(mds...) {
		lowerK := strings.ToLower(k)
		// binary headers must be base-64-encoded
		isBin := strings.HasSuffix(lowerK, "-bin")
		for _, v := range vs {
			if isBin {
				v = base64.URLEncoding.EncodeToString([]byte(v))
			}
			h = append(h, &goatorepo.KeyValue{Key: k, Value: v})
		}
	}
	return h
}

func ToMetadata(kvs []*goatorepo.KeyValue) (metadata.MD, error) {
	md := metadata.MD{}
	for _, h := range kvs {
		k := strings.ToLower(h.Key)
		v := h.Value
		if strings.HasSuffix(k, "-bin") {
			vv, err := base64.URLEncoding.DecodeString(v)
			if err != nil {
				return nil, err
			}
			v = string(vv)
		}
		md[k] = append(md[k], v)
	}
	return md, nil
}

// NewFnReadWriter is a convenience wrapper to turn read and write functions
// into an RpcReadWriter.
func NewFnReadWriter(
	r func(context.Context) (*goatorepo.Rpc, error),
	w func(context.Context, *goatorepo.Rpc) error,
) types.RpcReadWriter {
	return &fnReadWriter{r, w}
}

type fnReadWriter struct {
	r func(context.Context) (*goatorepo.Rpc, error)
	w func(context.Context, *goatorepo.Rpc) error
}

func (frw *fnReadWriter) Read(ctx context.Context) (*goatorepo.Rpc, error) {
	return frw.r(ctx)
}

func (frw *fnReadWriter) Write(ctx context.Context, rpc *goatorepo.Rpc) error {
	return frw.w(ctx, rpc)
}

func StatsStartServerRPC(
	statsHandlers []stats.Handler,
	isClient bool,
	beginTime time.Time,
	method string,
	isClientStream bool,
	isServerStream bool,
	ctx context.Context,
) context.Context {
	if len(statsHandlers) == 0 {
		return ctx
	}

	incomingMetadata, imOk := metadata.FromIncomingContext(ctx)
	if !imOk {
		incomingMetadata = metadata.MD{}
	}

	for _, sh := range statsHandlers {
		ctx = sh.TagRPC(ctx, &stats.RPCTagInfo{
			FullMethodName: method,
		})
		statsBegin := &stats.Begin{
			Client:         isClient,
			BeginTime:      beginTime,
			IsClientStream: isClientStream,
			IsServerStream: isServerStream,
		}
		sh.HandleRPC(ctx, statsBegin)

		if !isClient {
			sh.HandleRPC(ctx, &stats.InHeader{
				Client:     isClient,
				FullMethod: method,
				// Note regular GRPC fills in more fields like so. We don't have this information
				// given the way GOAT works (the underlying transport is abstracted away), so we
				// leave it all as zero.
				//RemoteAddr:  t.Peer().Addr,
				//LocalAddr:   t.Peer().LocalAddr,
				//Compression: stream.RecvCompress(),
				//WireLength:  stream.HeaderWireLength(),
				Header: incomingMetadata,
			})
		}
	}
	return ctx
}

func StatsEndRPC(
	statsHandlers []stats.Handler,
	isClient bool,
	beginTime time.Time,
	appErr error,
	ctx context.Context,
) {
	for _, sh := range statsHandlers {
		end := &stats.End{
			Client:    isClient,
			BeginTime: beginTime,
			EndTime:   time.Now(),
		}
		if appErr != nil && !errors.Is(appErr, io.EOF) {
			end.Error = appErr
		}
		sh.HandleRPC(ctx, end)
	}
}
