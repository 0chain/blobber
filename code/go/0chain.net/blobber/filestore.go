package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

func setupFileStore() (err error) {
	fmt.Print("[9/12] setup file store")
	if isIntegrationTest {
		filestore.SetIsMountPointFunc(func(s string) bool { return true })
	}
	return filestore.SetupFSStore(config.Configuration.MountPoint)
}
