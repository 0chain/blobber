//go:build !integration_tests
// +build !integration_tests

package main

// the following prepare functions are noop

func prepareBlobber(id string) {}

func prepareBlobberShutdown() {} //nolint:unused,deadcode // looks like it is being used in integration test
