package writemarker

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startRedeemWriteMarkers(ctx)
}

func redeemWriterMarkersForAllocation(allocationObj *allocation.Allocation) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] redeemWriterMarkersForAllocation", zap.Any("err", r))
		}
	}()

	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	db := datastore.GetStore().GetTransaction(ctx)
	var err error

	done := false

	defer func() {

		if !done {
			if err := db.Rollback().Error; err != nil {
				Logger.Error("Error rollbacking the writemarker redeem",
					zap.Any("allocation", allocationObj.ID),
					zap.Error(err))
			}
		}
		ctx.Done()
	}()

	var writemarkers []*WriteMarkerEntity

	err = db.Not(WriteMarkerEntity{Status: Committed}).
		Where(WriteMarker{AllocationID: allocationObj.ID}).
		Order("sequence").
		Find(&writemarkers).Error
	if err != nil {
		Logger.Error("Error redeeming the write marker. failed to load allocation's writemarker ",
			zap.Any("allocation", allocationObj.ID),
			zap.Any("error", err))
		return
	}
	startredeem := false
	for _, wm := range writemarkers {
		if wm.WM.PreviousAllocationRoot == allocationObj.LatestRedeemedWM && !startredeem {
			startredeem = true
		}
		if startredeem || allocationObj.LatestRedeemedWM == "" {
			err = wm.RedeemMarker(ctx)
			if err != nil {
				Logger.Error("Error redeeming the write marker.",
					zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))
				return
			}
			err = db.Model(allocationObj).Updates(allocation.Allocation{LatestRedeemedWM: wm.WM.AllocationRoot}).Error
			if err != nil {
				Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed",
					zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))

				return
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

	err = db.Commit().Error
	if err != nil {
		Logger.Error("Error committing the writemarker redeem",
			zap.Any("allocation", allocationObj.ID),
			zap.Error(err))
	}

	done = true
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
			Logger.Info("Trying to redeem writemarkers.",
				zap.Any("numOfWorkers", config.Configuration.WMRedeemNumWorkers))
			redeemWriteMarkers()
		}
	}
}
