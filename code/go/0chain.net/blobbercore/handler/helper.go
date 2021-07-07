package handler

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
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
