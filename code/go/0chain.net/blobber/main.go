package main

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func main() {

	parseFlags()

	setupConfig(configDir, deploymentMode)

	setupLogging()

	if err := setupDatabase(); err != nil {
		logging.Logger.Error("Error setting up data store" + err.Error())
		panic(err)
	}

	if err := reloadConfig(); err != nil {
		logging.Logger.Error("Error reloading config" + err.Error())
		panic(err)
	}

	if err := setupNode(); err != nil {
		logging.Logger.Error("Error setting up blobber node " + err.Error())
		panic(err)
	}

	if err := setupServerChain(); err != nil {
		logging.Logger.Error("Error setting up server chain " + err.Error())
		panic(err)
	}

	// Initialize after server chain is setup.
	if err := setupFileStore(); err != nil {
		logging.Logger.Error("Error setting up file store" + err.Error())
		panic(err)
	}

	// prepare is to configure more.
	// when enabled "// +build integration_tests", this sets blobber for conductor tests.
	prepareBlobber(node.Self.ID)
	common.SetBlobberRegistered(true)
	go func() {
		if err := registerOnChain(); err != nil {
			logging.Logger.Error("Error register on blockchain" + err.Error())
		}
	}()

	// todo: activate this when gRPC functionalities are implemented
	// go startGRPCServer()

	startHttpServer()
}
