package writemarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

func startRedeemWorker(ctx context.Context) {
	logging.Logger.Info("Starting redeem worker")
	for i := 0; i < config.Configuration.WMRedeemNumWorkers; i++ {
		go redeemWorker(ctx)
	}
}

func redeemWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("Stopping redeem worker")
			return
		case dm := <-writeMarkerChan:
			_ = redeemWriteMarker(dm)
		}
	}
}
