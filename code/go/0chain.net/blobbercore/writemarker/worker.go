package writemarker

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
)

var (
	writeMarkerChan chan *WriteMarkerEntity
	writeMarkerMap  map[string]*semaphore.Weighted
	mut             sync.RWMutex
)

func SetupWorkers(ctx context.Context) {
	var res []allocation.Res

	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		res = allocation.Repo.GetAllocationIds(ctx)
		return nil
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Error("error_getting_allocations_worker",
			zap.Any("error", err))
	}

	writeMarkerMap = make(map[string]*semaphore.Weighted)

	for _, r := range res {
		writeMarkerMap[r.ID] = semaphore.NewWeighted(1)
	}

	go startRedeem(ctx)
}

func GetLock(allocationID string) *semaphore.Weighted {
	mut.RLock()
	defer mut.RUnlock()
	return writeMarkerMap[allocationID]
}

func SetLock(allocationID string) *semaphore.Weighted {
	mut.Lock()
	defer mut.Unlock()
	writeMarkerMap[allocationID] = semaphore.NewWeighted(1)
	return writeMarkerMap[allocationID]
}

func redeemWriteMarker(wm *WriteMarkerEntity) error {
	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	db := datastore.GetStore().GetTransaction(ctx)
	allocationID := wm.WM.AllocationID
	shouldRollback := false
	start := time.Now()
	defer func() {
		if shouldRollback {
			if rollbackErr := db.Rollback().Error; rollbackErr != nil {
				logging.Logger.Error("Error rollback on redeeming the write marker.",
					zap.Any("allocation", allocationID),
					zap.Any("wm", wm.WM.AllocationID), zap.Error(rollbackErr))
			}
		}
	}()
	alloc, err := allocation.Repo.GetByIdAndLock(ctx, allocationID)
	now := time.Now()
	logging.Logger.Info("[writemarker]Lock Allocation", zap.Int64("now", now.Unix()), zap.String("allocation_root", wm.WM.AllocationRoot))
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker.", zap.Any("allocation", allocationID), zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))
		go tryAgain(wm)
		shouldRollback = true
		return err
	}

	if alloc.AllocationRoot != wm.WM.AllocationRoot {
		logging.Logger.Info("Stale write marker. Allocation root mismatch",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("alloc_root", alloc.AllocationRoot))
		_ = wm.UpdateStatus(ctx, Rollbacked, "rollbacked", "")
		err = db.Commit().Error
		mut := GetLock(allocationID)
		if mut != nil {
			mut.Release(1)
		}
		return err
	}

	err = wm.RedeemMarker(ctx)
	if err != nil {
		elapsedTime := time.Since(start)
		logging.Logger.Error("Error redeeming the write marker.",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm), zap.Any("error", err), zap.Any("elapsedTime", elapsedTime))
		go tryAgain(wm)
		shouldRollback = true

		return err
	}

	// err = allocation.Repo.UpdateAllocationRedeem(ctx, wm.WM.AllocationRoot, allocationID)
	// alloc.LatestRedeemedWM = wm.WM.AllocationRoot
	// alloc.IsRedeemRequired = false
	// err = allocation.Repo.Save(ctx, alloc)
	affected := db.Exec("UPDATE allocations SET latest_redeemed_write_marker = ?, is_redeem_required = ? WHERE id = ?", wm.WM.AllocationRoot, false, allocationID).RowsAffected
	now = time.Now()
	logging.Logger.Info("[writemarker]Update Allocation", zap.Int64("now", now.Unix()), zap.String("allocation_root", wm.WM.AllocationRoot), zap.Int64("affected", affected))
	if err != nil {
		mut := GetLock(allocationID)
		if mut != nil {
			mut.Release(1)
		}
		logging.Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))
		shouldRollback = true
		return err
	}

	err = db.Commit().Error
	now = time.Now()
	logging.Logger.Info("[writemarker]Unlock Allocation", zap.Int64("now", now.Unix()), zap.String("allocation_root", wm.WM.AllocationRoot))
	mut := GetLock(allocationID)
	if mut != nil {
		mut.Release(1)
	}
	if err != nil {
		logging.Logger.Error("Error committing the writemarker redeem",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Error(err))
		shouldRollback = true
		return err
	}
	elapsedTime := time.Since(start)
	logging.Logger.Info("Success Redeeming the write marker",
		zap.Any("allocation", allocationID),
		zap.Any("wm", wm.WM.AllocationRoot), zap.Any("txn", wm.CloseTxnID), zap.Any("elapsedTime", elapsedTime))

	return nil
}

func startRedeem(ctx context.Context) {
	logging.Logger.Info("Start redeeming writemarkers")
	writeMarkerChan = make(chan *WriteMarkerEntity, 200)
	go startRedeemWorker(ctx)

	var writemarkers []*WriteMarkerEntity
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		return tx.Not(WriteMarkerEntity{Status: Committed}).Find(&writemarkers).Error
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Error("Error redeeming the write marker. failed to load allocation's writemarker ",
			zap.Any("error", err))
		return
	}

	for _, wm := range writemarkers {
		mut := GetLock(wm.WM.AllocationID)
		if mut == nil {
			mut = SetLock(wm.WM.AllocationID)
		}
		err := mut.Acquire(ctx, 1)
		if err != nil {
			logging.Logger.Error("Error acquiring semaphore", zap.Error(err))
			continue
		}
		writeMarkerChan <- wm
	}

}

func tryAgain(wm *WriteMarkerEntity) {
	writeMarkerChan <- wm
}
