//go:build integration_tests
// +build integration_tests

package main

import (
	crpc "github.com/0chain/blobber/code/go/0chain.net/conductor/conductrpc" // integration tests
)

func prepare(id string) {
	crpc.Init(id)
}
