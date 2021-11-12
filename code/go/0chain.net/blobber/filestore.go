package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

var fsStore filestore.FileStore //nolint:unused // global which might be needed somewhere

func setupFileStore() (err error) {
	fmt.Print("[8/11] setup file store")

	fsStore, err = filestore.SetupFSStore(filesDir + "/files")

	fmt.Print("		[OK]\n")

	return err
}
