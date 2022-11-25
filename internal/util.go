package internal

import (
	"encoding/base64"
	"strings"

	"google.golang.org/grpc/metadata"

	wrapped "github.com/avos-io/goat/gen"
)

func ToKeyValue(mds ...metadata.MD) []*wrapped.KeyValue {
	h := []*wrapped.KeyValue{}
	for k, vs := range metadata.Join(mds...) {
		lowerK := strings.ToLower(k)
		// binary headers must be base-64-encoded
		isBin := strings.HasSuffix(lowerK, "-bin")
		for _, v := range vs {
			if isBin {
				v = base64.URLEncoding.EncodeToString([]byte(v))
			}
			h = append(h, &wrapped.KeyValue{Key: k, Value: v})
		}
	}
	return h
}

func ToMetadata(kvs []*wrapped.KeyValue) (metadata.MD, error) {
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
