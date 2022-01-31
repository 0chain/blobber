package writemarker

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startRedeemWriteMarkers(ctx)
}

func RedeemMarkersForAllocation(ctx context.Context, allocationObj *allocation.Allocation) error {
	rctx := datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(rctx)
	defer func() {
		err := db.Commit().Error
		if err != nil {
			Logger.Error("Error committing the writemarker redeem", zap.Error(err))
		}
		rctx.Done()
	}()

	writemarkers := make([]*WriteMarkerEntity, 0)

	err := db.Not(WriteMarkerEntity{Status: Committed}).
		Where(WriteMarker{AllocationID: allocationObj.ID}).
		Order("sequence").
		Find(&writemarkers).Error
	if err != nil {
		return err
	}
	startredeem := false
	for _, wm := range writemarkers {
		if wm.WM.PreviousAllocationRoot == allocationObj.LatestRedeemedWM && !startredeem {
			startredeem = true
		}
		if startredeem || allocationObj.LatestRedeemedWM == "" {
			err := wm.RedeemMarker(rctx)
			if err != nil {
				Logger.Error("Error redeeming the write marker.", zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))
				continue
			}
			err = db.Model(allocationObj).Updates(allocation.Allocation{LatestRedeemedWM: wm.WM.AllocationRoot}).Error
			if err != nil {
				Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed", zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))
				return err
			}
			allocationObj.LatestRedeemedWM = wm.WM.AllocationRoot
			Logger.Info("Success Redeeming the write marker", zap.Any("wm", wm.WM.AllocationRoot), zap.Any("txn", wm.CloseTxnID))
		}
	}
	if allocationObj.LatestRedeemedWM == allocationObj.AllocationRoot {
		db.Model(allocationObj).
			Where("allocation_root = ? AND allocation_root = latest_redeemed_write_marker", allocationObj.AllocationRoot).
			Update("is_redeem_required", false)
	}
	return nil
}

func startRedeemWriteMarkers(ctx context.Context) {
	var ticker = time.NewTicker(
		time.Duration(config.Configuration.WMRedeemFreq) * time.Second,
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Logger.Info("Trying to redeem writemarkers.",
			//	zap.Any("numOfWorkers", numOfWorkers))
			redeemWriteMarker(ctx)
		}
	}

}
