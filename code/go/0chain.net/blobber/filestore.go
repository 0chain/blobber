package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

func setupFileStore() (err error) {
	fmt.Print("> setup file store")
	var fs filestore.FileStorer
	if isIntegrationTest {
		fs = &filestore.MockStore{}
	} else {
		fs = &filestore.FileStore{}

	}

	err = fs.Initialize()
	if err != nil {
		return
	}

	filestore.SetFileStore(fs)

	fmt.Print("	[OK]\n")
	return nil
}
