// +build integration_tests

package main

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/gorilla/mux"
)

func registerGRPCServer(r *mux.Router) {
	grpcServer := handler.NewGRPCServerWithMiddlewares(r)
	reflection.Register(grpcServer)
}
