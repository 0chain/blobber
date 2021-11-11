// +build !integration_tests

package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/mux"
	"google.golang.org/grpc/reflection"
)

func startGRPCServer() {
	fmt.Println("[10/11] starting grpc server	[OK]")

	r := mux.NewRouter()

	common.ConfigRateLimits()
	initHandlers(r)
	grpcServer := handler.NewGRPCServerWithMiddlewares(r)
	reflection.Register(grpcServer)

	if grpcPort <= 0 {
		logging.Logger.Error("grpc port missing")
		return
	}

	logging.Logger.Info("started grpc server on to grpc requests on port - " + strconv.Itoa(grpcPort))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Fatal(grpcServer.Serve(lis))
}
