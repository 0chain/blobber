package writemarker

import (
	"context"
	"strings"
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

// const (
// 	timestampGap          = 30 * 24 * 60 * 60  // 30 days
// 	cleanupWorkerInterval = 24 * 7 * time.Hour // 7 days
// )

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
	// go startCleanupWorker(ctx)
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
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker.", zap.Any("allocation", allocationID), zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err))
		if err != gorm.ErrRecordNotFound {
			go tryAgain(wm)
		}
		shouldRollback = true
		return err
	}

	if alloc.Finalized {
		logging.Logger.Info("Allocation is finalized. Skipping redeeming the write marker.", zap.Any("allocation", allocationID), zap.Any("wm", wm.WM.AllocationID))
		shouldRollback = true
		return nil
	}

	if alloc.AllocationRoot != wm.WM.AllocationRoot {
		logging.Logger.Info("Stale write marker. Allocation root mismatch",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("alloc_root", alloc.AllocationRoot))
		if wm.ReedeemRetries == 0 && !alloc.IsRedeemRequired {
			wm.ReedeemRetries++
			go tryAgain(wm)
			shouldRollback = true
			return nil
		}
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
		if retryRedeem(err.Error()) {
			go tryAgain(wm)
		} else {
			mut := GetLock(allocationID)
			if mut != nil {
				mut.Release(1)
			}
		}
		shouldRollback = true

		return err
	}
	defer func() {
		mut := GetLock(allocationID)
		if mut != nil {
			mut.Release(1)
		}
	}()
	err = allocation.Repo.UpdateAllocationRedeem(ctx, allocationID, wm.WM.AllocationRoot, alloc)
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))
		shouldRollback = true
		return err
	}

	err = db.Commit().Error
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
	time.Sleep(time.Duration(wm.ReedeemRetries) * 5 * time.Second)
	writeMarkerChan <- wm
}

// Can add more cases where we don't want to retry
func retryRedeem(errString string) bool {
	return !strings.Contains(errString, "value not present")
}

// TODO: don't delete prev WM
// func startCleanupWorker(ctx context.Context) {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-time.After(cleanupWorkerInterval):
// 			_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
// 				tx := datastore.GetStore().GetTransaction(ctx)
// 				timestamp := int64(common.Now()) - timestampGap // 30 days
// 				err := tx.Exec("INSERT INTO write_markers_archive (SELECT * from write_markers WHERE timestamp < ? AND latest = )", timestamp, false).Error
// 				if err != nil {
// 					return err
// 				}
// 				return tx.Exec("DELETE FROM write_markers WHERE timestamp < ? AND latest = )", timestamp, false).Error
// 			})
// 		}
// 	}
// }
