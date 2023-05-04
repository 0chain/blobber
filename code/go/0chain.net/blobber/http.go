package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync"
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

	r := mux.NewRouter()
	initHandlers(r)

	var wg sync.WaitGroup

	wg.Add(2)
	// start http server
	go startServer(&wg, r, mode, httpPort, false)
	// start https server
	go startServer(&wg, r, mode, httpsPort, true)

	logging.Logger.Info("Ready to listen to the requests")
	fmt.Print("> start http server	[OK]\n")

	wg.Wait()
}

func startServer(wg *sync.WaitGroup, r *mux.Router, mode string, port int, isTls bool) {
	defer wg.Done()

	if port <= 0 {
		return
	}

	logging.Logger.Info("Starting blobber", zap.Int("available_cpus", runtime.NumCPU()), zap.Int("port", port), zap.Bool("is-tls", isTls), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))

	//address := publicIP + ":" + portString
	address := ":" + strconv.Itoa(port)
	var server *http.Server

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

	if isTls {
		err := server.ListenAndServeTLS(httpsCertFile, httpsKeyFile)
		logging.Logger.Fatal("validator failed", zap.Error(err))
	} else {
		err := server.ListenAndServe()
		logging.Logger.Fatal("validator failed", zap.Error(err))
	}
}

func initHandlers(r *mux.Router) {
	handler.StartTime = time.Now().UTC()
	r.HandleFunc("/", handler.HomepageHandler)
	handler.SetupHandlers(r)
	handler.SetupSwagger()
	common.SetAdminCredentials()
}
