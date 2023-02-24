package main

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/challenge"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/handler"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func setupWorkers(ctx context.Context) {
	handler.SetupWorkers(ctx)
	challenge.SetupWorkers(ctx)
	readmarker.SetupWorkers(ctx)
	writemarker.SetupWorkers(ctx)
	allocation.StartUpdateWorker(ctx, config.Configuration.UpdateAllocationsInterval)
	updateCCTWorker(ctx)
}

func refreshPriceOnChain(ctx context.Context) {
	var REPEAT_DELAY = 60 * 60 * time.Duration(viper.GetInt("price_worker_in_hours")) // 12 hours with default settings
	var err error
	for {
		select {
		case <-ctx.Done():
			return

		case <-time.After(REPEAT_DELAY * time.Second):
			err = handler.RefreshPriceOnChain(common.GetRootContext())
			if err != nil {
				logging.Logger.Error("refresh price on chain ", zap.Error(err))
			}
		}

	}
}
