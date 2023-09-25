package main

import (
	"fmt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/zcncore"
)

func setupLogging() {
	fmt.Print("> init logging")

	if config.Development() {
		logging.InitLogging("development", logDir, "0chainBlobber.log")
	} else {
		logging.InitLogging("production", logDir, "0chainBlobber.log")
	}

	zcncore.SetLogFile(logDir+"/0chainBlobber-sdk.log", false)
	zcncore.SetLogLevel(3)
	fmt.Print("		[OK]\n")
}
