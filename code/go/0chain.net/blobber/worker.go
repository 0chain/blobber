package main

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

func setupWorkers(ctx context.Context) {
	handler.SetupWorkers(ctx)
	challenge.SetupWorkers(ctx)
	readmarker.SetupWorkers(ctx)
	writemarker.SetupWorkers(ctx)
	// allocation.StartUpdateWorker(ctx, config.Configuration.UpdateAllocationsInterval)
	allocation.SetupWorkers(ctx)
	updateCCTWorker(ctx)
}

// startRefreshSettings sync settings from blockchain
func startRefreshSettings(ctx context.Context) {
	const REPEAT_DELAY = 60 * 3 // 3 minutes
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(REPEAT_DELAY * time.Second):
			_, err = config.ReloadFromChain(common.GetRootContext(), datastore.GetStore().GetDB())
			if err != nil {
				logging.Logger.Warn("failed to refresh blobber settings from chain", zap.Error(err))
				continue
			}

			logging.Logger.Info("success to refresh blobber settings from chain")

		}

	}
}
