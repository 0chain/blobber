package main

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
)

const totalSteps = 12

func main() {
	parseFlags(1)

	setupConfig(2, configDir, deploymentMode)

	if err := setupDatabase(3); err != nil {
		logging.Logger.Error("Error setting up data store" + err.Error())
		panic(err)
	}

	reloadConfigFromDatastore(4)

	setupLogging(5)

	if err := setupMinio(6); err != nil {
		logging.Logger.Error("Error setting up minio " + err.Error())
		panic(err)
	}

	if err := setupNode(7); err != nil {
		logging.Logger.Error("Error setting up blobber node " + err.Error())
		panic(err)
	}

	if err := setupServerChain(8); err != nil {
		logging.Logger.Error("Error setting up server chain" + err.Error())
		panic(err)
	}

	// Initialize after server chain is setup.
	if err := setupFileStore(9); err != nil {
		logging.Logger.Error("Error setting up file store" + err.Error())
		panic(err)
	}

	// only enabled with "// +build integration_tests"
	initIntegrationsTests(node.Self.ID)

	go setupOnChain(10)

	startGRPCServer(11)

	startHttpServer(12)
}
