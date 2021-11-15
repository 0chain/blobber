package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

func main() {
	parseFlags()

	setupConfig()

	setupLogging()

	if err := setupMinio(); err != nil {
		logging.Logger.Error("Error setting up minio " + err.Error())
		panic(err)
	}

	if err := setupNode(); err != nil {
		logging.Logger.Error("Error setting up blobber node " + err.Error())
		panic(err)
	}

	if !isIntegrationTest {
		if err := setupServerChain(); err != nil {
			logging.Logger.Error("Error setting up server chain" + err.Error())
			panic(err)
		}
	} else {
		fmt.Print("[6/10] setup server chain	[SKIP]\n")
	}

	if err := setupDatabase(); err != nil {
		logging.Logger.Error("Error setting up data store" + err.Error())
		panic(err)
	}

	// Initialize after server chain is setup.
	if err := setupFileStore(); err != nil {
		logging.Logger.Error("Error setting up file store" + err.Error())
		panic(err)
	}

	// only enabled with "// +build integration_tests"
	initIntegrationsTests(node.Self.ID)

	if !isIntegrationTest {
		go setupOnChain()
	} else {
		fmt.Print("[9/11] connecting to chain	[SKIP]\n")
	}

	startGRPCServer()

	startHttpServer()

}
