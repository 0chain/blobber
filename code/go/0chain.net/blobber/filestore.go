package main

import (
	"fmt"

	disk_balancer "github.com/0chain/blobber/code/go/0chain.net/blobbercore/disk-balancer"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

var fsStore filestore.FileStore //nolint:unused // global which might be needed somewhere

func setupFileStore() (err error) {
	fmt.Print("[8/11] setup file store")

	disk_balancer.NewDiskSelector()
	fdir, _ := disk_balancer.GetDiskSelector().GetNextVolumePath(0)
	fsStore, err = filestore.SetupFSStore(fdir + "/files")

	fmt.Print("		[OK]\n")

	return err
}
