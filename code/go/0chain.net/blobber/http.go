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
	"github.com/0chain/blobber/code/go/0chain.net/core/build"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zcncore"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

var startTime time.Time

func startHttpServer() {
	fmt.Println("[11/11] start http server	[OK]")

	mode := "main net"
	if config.Development() {
		mode = "development"
	} else if config.TestNet() {
		mode = "test net"
	}

	logging.Logger.Info("Starting blobber", zap.Int("available_cpus", runtime.NumCPU()), zap.Int("port", httpPort), zap.String("chain_id", config.GetServerChainID()), zap.String("mode", mode))

	//address := publicIP + ":" + portString
	address := ":" + strconv.Itoa(httpPort)
	var server *http.Server

	r := mux.NewRouter()

	common.ConfigRateLimits()
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

	startTime = time.Now().UTC()

	log.Fatal(server.ListenAndServe())
}

func initHandlers(r *mux.Router) {
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mc := chain.GetServerChain()

		fmt.Fprintf(w, "<div>Running since %v ...\n", startTime)
		fmt.Fprintf(w, "<div>Working on the chain: %v</div>\n", mc.ID)
		fmt.Fprintf(w,
			"<div>I am a blobber with <ul><li>id:%v</li><li>public_key:%v</li><li>build_tag:%v</li></ul></div>\n",
			node.Self.ID, node.Self.PublicKey, build.BuildTag,
		)

		fmt.Fprintf(w, "<div>Miners ...\n")
		network := zcncore.GetNetwork()
		for _, miner := range network.Miners {
			fmt.Fprintf(w, "%v\n", miner)
		}

		fmt.Fprintf(w, "<div>Sharders ...\n")
		for _, sharder := range network.Sharders {
			fmt.Fprintf(w, "%v\n", sharder)
		}
	})

	handler.SetupHandlers(r)
}
