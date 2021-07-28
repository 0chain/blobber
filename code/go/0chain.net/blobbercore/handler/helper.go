package handler

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

func registerGRPCServices(r *mux.Router, server *grpc.Server) {
	blobberService := newGRPCBlobberService()
	grpcGatewayHandler := runtime.NewServeMux()

	blobbergrpc.RegisterBlobberServer(server, blobberService)
	_ = blobbergrpc.RegisterBlobberHandlerServer(context.Background(), grpcGatewayHandler, blobberService)
	r.PathPrefix("/").Handler(grpcGatewayHandler)

}

func checkValidDate(s string) error {
	if s != "" {
		_, err := time.Parse("2006-01-02 15:04:05.999999999", s)
		if err != nil {
			return common.NewError("invalid_parameters", err.Error())
		}
	}
	return nil
}
