//go:build integration_tests
// +build integration_tests

// Integration tests is also called conductor tests.
// TODO. There seems to be missing setup for conductor as only prepareBlobber() is hooked.

package main

import (
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	crpc "github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc" // integration tests
)

// start lock, where the miner is ready to connect to blockchain (BC)
func prepareBlobber(id string) {
	logging.Logger.Info("integration tests")
	crpc.Init(id)
}

func prepareBlobberShutdown() {
	crpc.Shutdown()
}
