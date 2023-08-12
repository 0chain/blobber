package main

import (
	"fmt"
	"net/http"
	"net/http/pprof"
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
	var profServer *http.Server

	if config.Development() {
		// No WriteTimeout setup to enable pprof
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       30 * time.Second,
			MaxHeaderBytes:    1 << 20,
			Handler:           r,
		}

		pprofMux := http.NewServeMux()
		profServer = &http.Server{
			Addr:           fmt.Sprintf(":%d", port-1000),
			ReadTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20,
			Handler:        pprofMux,
		}
		initProfHandlers(pprofMux)
		go func() {
			err2 := profServer.ListenAndServe()
			logging.Logger.Error("Http server shut down", zap.Error(err2))
		}()

	} else {
		server = &http.Server{
			Addr:              address,
			ReadHeaderTimeout: 30 * time.Second,
			ReadTimeout:       30 * time.Second,
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

func initProfHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", handler.RateLimitByGeneralRL(pprof.Index))
	mux.HandleFunc("/debug/pprof/cmdline", handler.RateLimitByGeneralRL(pprof.Cmdline))
	mux.HandleFunc("/debug/pprof/profile", handler.RateLimitByGeneralRL(pprof.Profile))
	mux.HandleFunc("/debug/pprof/symbol", handler.RateLimitByGeneralRL(pprof.Symbol))
	mux.HandleFunc("/debug/pprof/trace", handler.RateLimitByGeneralRL(pprof.Trace))
}
