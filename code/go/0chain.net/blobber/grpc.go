// +build !integration_tests

package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/reflection"
)

func startGRPCServer(r mux.Router) {
	grpcServer := handler.NewGRPCServerWithMiddlewares(&r)
	reflection.Register(grpcServer)

	if grpcPort <= 0 {
		logging.Logger.Error("Could not start grpc server since grpc port has not been specified." +
			" Please specify the grpc port in the --grpc_port build arguement to start the grpc server")
		return
	}

	logging.Logger.Info("listening too grpc requests on port - " + strconv.Itoa(grpcPort))
	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Fatal(grpcServer.Serve(lis))
}
