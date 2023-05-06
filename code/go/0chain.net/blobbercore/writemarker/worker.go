package writemarker

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"golang.org/x/sync/semaphore"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go.uber.org/zap"
)

var writeMarkerChan chan *WriteMarkerEntity

var writeMarkerMap map[string]*semaphore.Weighted
var mut = &sync.RWMutex{}

func SetupWorkers(ctx context.Context) {

	db := datastore.GetStore().GetDB()
	type Res struct {
		ID string
	}
	var res []Res

	err := db.Model(&allocation.Allocation{}).Select("id").Find(&res).Error
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
	logging.Logger.Info("Redeeming the write marker.", zap.String("allocation", allocationID))
	defer func() {
		if shouldRollback {
			if rollbackErr := db.Rollback().Error; rollbackErr != nil {
				logging.Logger.Error("Error rollback on redeeming the write marker.",
					zap.Any("allocation", allocationID),
					zap.Any("wm", wm.WM.AllocationID), zap.Error(rollbackErr))
			}
		}
	}()
	alloc := &allocation.Allocation{}
	err := db.Model(&allocation.Allocation{}).Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Select("allocation_root").Where("id=?", allocationID).First(alloc).Error
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
		mut := GetLock(allocationID)
		if mut != nil {
			mut.Release(1)
		}
		_ = wm.UpdateStatus(ctx, Rollbacked, "", "")
		err = db.Commit().Error
		return err
	}

	err = wm.RedeemMarker(ctx)
	if err != nil {
		elapsedTime := time.Since(start)
		logging.Logger.Error("Error redeeming the write marker.",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationID), zap.Any("error", err), zap.Any("elapsedTime", elapsedTime))
		go tryAgain(wm)
		shouldRollback = true

		return err
	}
	mut := GetLock(allocationID)
	if mut != nil {
		mut.Release(1)
	}

	err = db.Exec("UPDATE allocations SET latest_redeemed_write_marker=?,is_redeem_required=(allocation_root NOT LIKE ?) WHERE id=?",
		wm.WM.AllocationRoot, wm.WM.AllocationRoot, allocationID).Error

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
	logging.Logger.Info("Start Redeem writemarkers")
	writeMarkerChan = make(chan *WriteMarkerEntity, 200)
	go startRedeemWorker(ctx)
	db := datastore.GetStore().GetDB()

	var writemarkers []*WriteMarkerEntity

	err := db.Not(WriteMarkerEntity{Status: Committed}).Find(&writemarkers).Error
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
