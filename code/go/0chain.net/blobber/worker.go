package main

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func setupWorkers() {
	var root = common.GetRootContext()
	handler.SetupWorkers(root)
	challenge.SetupWorkers(root)
	readmarker.SetupWorkers(root)
	writemarker.SetupWorkers(root)
	allocation.StartUpdateWorker(root, config.Configuration.UpdateAllocationsInterval)
	if config.Configuration.AutomaticUpdate {
		go StartUpdateWorker(root, config.Configuration.BlobberUpdateInterval)
	}
}

func refreshPriceOnChain() {
	var REPEAT_DELAY = 60 * 60 * time.Duration(viper.GetInt("price_worker_in_hours")) // 12 hours with default settings
	for {
		time.Sleep(REPEAT_DELAY * time.Second)
		if err := handler.RefreshPriceOnChain(common.GetRootContext()); err != nil {
			logging.Logger.Error("refresh price on chain ", zap.Error(err))
		}
	}
}

func startHealthCheck() {
	const REPEAT_DELAY = 60 * 15 // 15 minutes
	var err error
	for {
		err = handler.SendHealthCheck()
		if err == nil {
			logging.Logger.Info("success to send heartbeat")
		} else {
			logging.Logger.Warn("failed to send heartbeat", zap.Error(err))
		}
		<-time.After(REPEAT_DELAY * time.Second)
	}
}

// startRefreshSettings sync settings from chain
func startRefreshSettings() {
	const REPEAT_DELAY = 60 * 15 // 15 minutes
	var err error
	for {
		err = config.Refresh(common.GetRootContext(), datastore.GetStore().GetDB())
		if err == nil {
			logging.Logger.Info("success to refresh blobber settings from chain")
		} else {
			logging.Logger.Warn("failed to refresh blobber settings from chain", zap.Error(err))
		}
		<-time.After(REPEAT_DELAY * time.Second)
	}
}

func StartUpdateWorker(ctx context.Context, interval time.Duration) {
	err := filestore.GetFileStore().CalculateCurrentDiskCapacity()
	if err != nil {
		panic(err)
	}
	currentCapacity := filestore.GetFileStore().GetCurrentDiskCapacity()

	ticker := time.NewTicker(config.Configuration.BlobberUpdateInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := filestore.GetFileStore().CalculateCurrentDiskCapacity()
			if err != nil {
				logging.Logger.Error("Error while getting capacity", zap.Error(err))
				break
			}
			if currentCapacity != filestore.GetFileStore().GetCurrentDiskCapacity() {

				err := handler.UpdateBlobberOnChain(ctx)
				if err != nil {
					logging.Logger.Error("Error while updating blobber updates on chain", zap.Error(err))
				}
			}
		}
	}

}
