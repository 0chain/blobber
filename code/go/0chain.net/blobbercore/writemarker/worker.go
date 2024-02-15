package writemarker

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	writeMarkerChan chan *markerData
	markerDataMap   map[string]*markerData
	markerDataMut   sync.Mutex
)

type markerData struct {
	firstMarkerTimestamp common.Timestamp
	allocationID         string
	retries              int
	chainLength          int
	processing           bool
}

func SaveMarkerData(allocationID string, timestamp common.Timestamp, chainLength int) {
	markerDataMut.Lock()
	defer markerDataMut.Unlock()
	if data, ok := markerDataMap[allocationID]; !ok {
		markerDataMap[allocationID] = &markerData{
			firstMarkerTimestamp: timestamp,
			allocationID:         allocationID,
			chainLength:          1,
		}
	} else {
		data.chainLength = chainLength
		if data.chainLength == 1 {
			data.firstMarkerTimestamp = timestamp
		}
		if !data.processing && (data.chainLength == MAX_CHAIN_LENGTH || common.Now()-data.firstMarkerTimestamp > MAX_TIMESTAMP_GAP) {
			data.processing = true
			writeMarkerChan <- data
		}
	}
}

func CheckProcessingMarker(allocationID string) bool {
	markerDataMut.Lock()
	defer markerDataMut.Unlock()
	if data, ok := markerDataMap[allocationID]; ok {
		return data.processing
	}
	return false
}

func deleteMarkerData(allocationID string) {
	markerDataMut.Lock()
	delete(markerDataMap, allocationID)
	markerDataMut.Unlock()
}

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

	startRedeem(ctx, res)
	go startCollector(ctx)
	// go startCleanupWorker(ctx)
}

func redeemWriteMarker(md *markerData) error {
	ctx := datastore.GetStore().CreateTransaction(context.TODO())
	db := datastore.GetStore().GetTransaction(ctx)
	allocationID := md.allocationID
	shouldRollback := false
	start := time.Now()
	defer func() {
		if shouldRollback {
			if rollbackErr := db.Rollback().Error; rollbackErr != nil {
				logging.Logger.Error("Error rollback on redeeming the write marker.",
					zap.Any("allocation", allocationID),
					zap.Error(rollbackErr))
			}

		} else {
			deleteMarkerData(allocationID)
		}
	}()

	res, _ := WriteMarkerMutext.Lock(ctx, allocationID, MARKER_CONNECTION)
	if res.Status != LockStatusOK {
		if common.Now()-md.firstMarkerTimestamp < 2*MAX_TIMESTAMP_GAP {
			md.retries++
			go tryAgain(md)
			shouldRollback = true
			return nil
		}
		//Exceeded twice of max timestamp gap, can be a malicious client keeping the lock forever to block the redeem
	} else {
		defer WriteMarkerMutext.Unlock(ctx, allocationID, MARKER_CONNECTION) //nolint:errcheck
	}
	allocMu := lock.GetMutex(allocation.Allocation{}.TableName(), allocationID)
	allocMu.RLock()
	defer allocMu.RUnlock()

	alloc, err := allocation.Repo.GetAllocationFromDB(ctx, allocationID)
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker.", zap.Any("allocation", allocationID), zap.Any("wm", allocationID), zap.Any("error", err))
		if err != gorm.ErrRecordNotFound {
			go tryAgain(md)
		}
		shouldRollback = true
		return err
	}

	if alloc.Finalized {
		logging.Logger.Info("Allocation is finalized. Skipping redeeming the write marker.", zap.Any("allocation", allocationID))
		deleteMarkerData(allocationID)
		shouldRollback = true
		return nil
	}

	wm, err := GetWriteMarkerEntity(ctx, alloc.AllocationRoot)
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker.", zap.Any("allocation", allocationID), zap.Any("wm", alloc.AllocationRoot), zap.Any("error", err))
		if err != gorm.ErrRecordNotFound {
			go tryAgain(md)
		}
		shouldRollback = true
		return err
	}

	err = wm.RedeemMarker(ctx)
	if err != nil {
		elapsedTime := time.Since(start)
		logging.Logger.Error("Error redeeming the write marker.",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm), zap.Any("error", err), zap.Any("elapsedTime", elapsedTime))
		if retryRedeem(err.Error()) {
			go tryAgain(md)
		}
		shouldRollback = true
		return err
	}
	deleteMarkerData(allocationID)

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

func startRedeem(ctx context.Context, res []allocation.Res) {
	logging.Logger.Info("Start redeeming writemarkers")
	writeMarkerChan = make(chan *markerData, 200)
	go startRedeemWorker(ctx)

	var writemarkers []*WriteMarkerEntity
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		for _, r := range res {
			wm := WriteMarkerEntity{}
			err := tx.Where("allocation_id = ?", r.ID).
				Order("sequence desc").
				Take(&wm).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if wm.WM.AllocationID != "" && wm.Status == Accepted {
				writemarkers = append(writemarkers, &wm)
			}
		}
		return nil
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Error("Error redeeming the write marker. failed to load allocation's writemarker ",
			zap.Any("error", err))
		return
	}

}

func tryAgain(md *markerData) {
	time.Sleep(time.Duration(md.retries) * 5 * time.Second)
	writeMarkerChan <- md
}

// Can add more cases where we don't want to retry
func retryRedeem(errString string) bool {
	return !strings.Contains(errString, "value not present")
}

func startCollector(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			markerDataMut.Lock()
			for _, data := range markerDataMap {
				if !data.processing && (data.chainLength == MAX_CHAIN_LENGTH || common.Now()-data.firstMarkerTimestamp > MAX_TIMESTAMP_GAP) {
					data.processing = true
					writeMarkerChan <- data
				}
			}
			markerDataMut.Unlock()
		}
	}
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
