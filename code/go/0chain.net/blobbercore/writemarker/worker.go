package writemarker

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startRedeemWriteMarkers(ctx)
}

func redeemWriterMarkersForAllocation(allocationObj *allocation.Allocation) {

	db := datastore.GetStore().GetDB()
	var err error

	var writemarkers []*WriteMarkerEntity

	err = db.Not(WriteMarkerEntity{Status: Committed}).
		Where(WriteMarker{AllocationID: allocationObj.ID}).
		Order("sequence").
		Find(&writemarkers).Error
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker. failed to load allocation's writemarker ",
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
			err = redeemWriteMarker(allocationObj, wm)
			if err != nil {
				return
			}
		}
	}

	if allocationObj.LatestRedeemedWM == allocationObj.AllocationRoot {
		err = db.Exec("UPDATE allocations SET is_redeem_required=? WHERE id = ? ", false, allocationObj.ID).Error
		if err != nil {
			logging.Logger.Error("Error redeeming the write marker. failed to update allocation's is_redeem_required ",
				zap.Any("allocation", allocationObj.ID),
				zap.Any("error", err))
		}
	}
}

func redeemWriteMarker(allocationObj *allocation.Allocation, wm *WriteMarkerEntity) error {
	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	db := datastore.GetStore().GetTransaction(ctx)

	shouldRollback := false

	defer func() {
		if shouldRollback {
			if err := db.Rollback(); err != nil {
				logging.Logger.Error("Error rollback on redeeming the write marker.",
					zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))
			}
		}
	}()

	err := wm.RedeemMarker(ctx)
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker.",
			zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))

		shouldRollback = true

		return err
	}

	err = db.Exec("UPDATE allocations SET latest_redeemed_write_marker=? WHERE id=?",
		wm.WM.AllocationRoot, allocationObj.ID).Error
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed",
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))
		shouldRollback = true
		return err
	}

	err = db.Commit().Error
	if err != nil {
		logging.Logger.Error("Error committing the writemarker redeem",
			zap.Any("allocation", allocationObj.ID),
			zap.Error(err))
		shouldRollback = true
		return err
	}

	allocationObj.LatestRedeemedWM = wm.WM.AllocationRoot
	logging.Logger.Info("Success Redeeming the write marker", zap.Any("wm", wm.WM.AllocationRoot), zap.Any("txn", wm.CloseTxnID))

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
			logging.Logger.Info("Trying to redeem writemarkers.",
				zap.Any("numOfWorkers", config.Configuration.WMRedeemNumWorkers))
			redeemWriteMarkers()
		}
	}
}
