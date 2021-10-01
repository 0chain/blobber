// +build !integration_tests

package main

import (
	"fmt"
	"log"
	"net"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/reflection"
)

func startGRPCServer(r mux.Router, port string) {
	grpcServer := handler.NewGRPCServerWithMiddlewares(&r)
	reflection.Register(grpcServer)

	if port == "" {
		logging.Logger.Error("Could not start grpc server since grpc port has not been specified." +
			" Please specify the grpc port in the --grpc_port build arguement to start the grpc server")
		return
	}

	logging.Logger.Info("listening too grpc requests on port - " + port)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Fatal(grpcServer.Serve(lis))

}
