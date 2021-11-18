// +build integration_tests

package main

import (
	"errors"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	crpc "github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc" // integration tests
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"log"
	"net"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/reflection"
)

// start lock, where the miner is ready to connect to blockchain (BC)
func initIntegrationsTests(id string) {
	logging.Logger.Info("integration tests")
	crpc.Init(id)
}

func shutdownIntegrationTests() {
	crpc.Shutdown()
}

func startGRPCServer() {
	common.ConfigRateLimits()
	r := mux.NewRouter()
	initHandlers(r)

	grpcServer := handler.NewGRPCServerWithMiddlewares(r)
	reflection.Register(grpcServer)

	if grpcPort <= 0 {
		logging.Logger.Error("grpc port missing")
		panic(errors.New("grpc port missing"))
	}

	logging.Logger.Info("started grpc server on to grpc requests on port - " + strconv.Itoa(grpcPort))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		panic(err)
	}

	fmt.Println("[10/11] starting grpc server	[OK]")
	go func() {
		log.Fatal(grpcServer.Serve(lis))
	}()
}
