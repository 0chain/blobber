package writemarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

func startRedeemWorker(ctx context.Context) {
	logging.Logger.Info("Starting redeem worker")
	sem := semaphore.NewWeighted(int64(config.Configuration.WMRedeemNumWorkers))
	for {
		select {
		case <-ctx.Done():
			logging.Logger.Info("Stopping redeem worker")
			return
		case wm := <-writeMarkerChan:
			logging.Logger.Info("Redeeming write marker", zap.String("allocation_root", wm.WM.AllocationRoot), zap.String("allocation_id", wm.WM.AllocationID))
			err := sem.Acquire(ctx, 1)
			if err == nil {
				go func() {
					_ = redeemWriteMarker(wm)
					sem.Release(1)
				}()
			}
		}
	}
}
