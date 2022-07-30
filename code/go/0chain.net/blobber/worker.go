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
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/zcncore"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func setupWorkers(ctx context.Context) {

	handler.SetupWorkers(ctx)
	challenge.SetupWorkers(ctx)
	readmarker.SetupWorkers(ctx)
	writemarker.SetupWorkers(ctx)
	allocation.StartUpdateWorker(ctx, config.Configuration.UpdateAllocationsInterval)
	if config.Configuration.AutomaticUpdate {
		go StartUpdateWorker(ctx, config.Configuration.BlobberUpdateInterval)
	}
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

func startHealthCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(config.Configuration.HealthCheckWorkerFreq):
			go func() {
				start := time.Now()

				txnHash, err := handler.SendHealthCheck()
				end := time.Now()
				if err == nil {
					logging.Logger.Info("success to send heartbeat", zap.String("txn_hash", txnHash), zap.Time("start", start), zap.Time("end", end), zap.Duration("duration", end.Sub(start)))
				} else {
					logging.Logger.Warn("failed to send heartbeat", zap.String("txn_hash", txnHash), zap.Time("start", start), zap.Time("end", end), zap.Duration("duration", end.Sub(start)))
				}
			}()
		}
	}
}

// startRefreshSettings sync settings from blockchain
func startRefreshSettings(ctx context.Context) {
	const REPEAT_DELAY = 60 * 3 // 3 minutes
	var err error
	var b *zcncore.Blobber
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(REPEAT_DELAY * time.Second):
			b, err = config.ReloadFromChain(common.GetRootContext(), datastore.GetStore().GetDB())
			if err != nil {
				logging.Logger.Warn("failed to refresh blobber settings from chain", zap.Error(err))
				continue
			}

			logging.Logger.Info("success to refresh blobber settings from chain")

			//	BaseURL is changed, register blobber to refresh it on blockchain again
			if b.BaseURL != node.Self.GetURLBase() {
				err = handler.UpdateBlobber(context.TODO())
				if err == nil {
					logging.Logger.Info("success to refresh blobber BaseURL on chain")
				} else {
					logging.Logger.Warn("failed to refresh blobber BaseURL on chain", zap.Error(err))
				}
			}
		}

	}
}

func StartUpdateWorker(ctx context.Context, interval time.Duration) {

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(config.Configuration.BlobberUpdateInterval):
			_, err := config.ReloadFromChain(common.GetRootContext(), datastore.GetStore().GetDB())

			if err != nil {
				logging.Logger.Error("Error while updating blobber updates on chain", zap.Error(err))
				continue
			}
		}
	}

}
