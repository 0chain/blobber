package handler

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"google.golang.org/grpc/metadata"
)

type GRPCMetaData struct {
	Client          string
	ClientKey       string
	ClientSignature string
}

func GetGRPCMetaDataFromCtx(ctx context.Context) *GRPCMetaData {
	metaData := &GRPCMetaData{}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return metaData
	}

	getMetaData := func(key string) string {
		list := md.Get(key)
		if len(list) > 0 {
			return list[0]
		}
		return ""
	}

	metaData.Client = getMetaData(common.ClientHeader)
	metaData.ClientKey = getMetaData(common.ClientKeyHeader)
	metaData.ClientSignature = getMetaData(common.ClientSignatureHeader)
	return metaData
}

func httpRequestWithMetaData(r *http.Request, md *GRPCMetaData, alloc string) {
	r.Header.Set(common.ClientHeader, md.Client)
	r.Header.Set(common.ClientKeyHeader, md.ClientKey)
	r.Header.Set(common.ClientSignatureHeader, md.ClientSignature)
	mux.SetURLVars(r, map[string]string{"allocation": alloc})
}
