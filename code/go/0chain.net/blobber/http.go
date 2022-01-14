package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func startHttpServer() {

	mode := "main net"
	if config.Development() {
		mode = "development"
	} else if config.TestNet() {
		mode = "test net"
	}

	portToUse := httpsPort
	if portToUse == 0 {
		portToUse = httpPort
	}

	logging.Logger.Info("Starting blobber", zap.Int("available_cpus", runtime.NumCPU()), zap.Int("port", portToUse), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))

	//address := publicIP + ":" + portString
	address := ":" + strconv.Itoa(portToUse)
	var server *http.Server

	common.ConfigRateLimits()
	r := mux.NewRouter()
	initHandlers(r)

	if config.Development() {
		// No WriteTimeout setup to enable pprof
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			MaxHeaderBytes:    1 << 20,
			Handler:           r,
		}
	} else {
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    1 << 20,
			Handler:           r,
		}
	}
	common.HandleShutdown(server)
	handler.HandleShutdown(common.GetRootContext())

	logging.Logger.Info("Ready to listen to the requests")
	fmt.Println("[11/11] start http server	[OK]")

	if portToUse == httpsPort {
		log.Fatal(server.ListenAndServeTLS(httpsCertFile, httpsKeyFile))
	} else {
		log.Fatal(server.ListenAndServe())
	}
}

func initHandlers(r *mux.Router) {
	handler.StartTime = time.Now().UTC()
	r.HandleFunc("/", handler.HomepageHandler)
	handler.SetupHandlers(r)
}
