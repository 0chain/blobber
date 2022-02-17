package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/disk-balancer"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

var fsStore filestore.FileStore //nolint:unused // global which might be needed somewhere

func setupFileStore() (err error) {
	fmt.Print("[9/12] setup file store")

	disk_balancer.StartDiskSelectorWorker(common.GetRootContext())
	root, err := disk_balancer.GetDiskSelector().GetNextDiskPath()
	if err != nil {
		return err
	}
	fsStore, err = filestore.SetupFSStore(root)

	fmt.Print("		[OK]\n")

	return err
}
