package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc/proto"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
)

type gRPCHeaderMetadata struct {
	Client          string
	ClientKey       string
	ClientSignature string
}

func registerGRPCServices(r *mux.Router, server *grpc.Server) {
	blobberService := newGRPCBlobberService()
	grpcGatewayHandler := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(CustomMatcher),
	)

	blobbergrpc.RegisterBlobberServiceServer(server, blobberService)
	_ = blobbergrpc.RegisterBlobberServiceHandlerServer(context.Background(), grpcGatewayHandler, blobberService)
	r.PathPrefix("/").Handler(grpcGatewayHandler)
}

func getGRPCMetaDataFromCtx(ctx context.Context) *gRPCHeaderMetadata {
	metaData := &gRPCHeaderMetadata{}

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

func httpRequestWithMetaData(r *http.Request, md *gRPCHeaderMetadata, alloc string) {
	r.Header.Set(common.ClientHeader, md.Client)
	r.Header.Set(common.ClientKeyHeader, md.ClientKey)
	r.Header.Set(common.ClientSignatureHeader, md.ClientSignature)
	*r = *mux.SetURLVars(r, map[string]string{"allocation": alloc})
}

func CustomMatcher(key string) (string, bool) {
	switch key {
	case common.ClientHeader:
		return key, true
	case common.ClientKeyHeader:
		return key, true
	case common.ClientSignatureHeader:
		return key, true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}
