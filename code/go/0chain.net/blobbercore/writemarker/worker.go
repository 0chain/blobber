package writemarker

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	writeMarkerChan chan *markerData
	markerDataMap   = make(map[string]*markerData)
	markerDataMut   sync.Mutex
)

type markerData struct {
	firstMarkerTimestamp common.Timestamp
	lastMarkerTimestamp  common.Timestamp
	allocationID         string
	retries              int
	chainLength          int
	processing           bool
	inCommit             bool // if new write marker is being committed
}

func SetCommittingMarker(allocationID string, committing bool) bool {
	markerDataMut.Lock()
	defer markerDataMut.Unlock()
	if data, ok := markerDataMap[allocationID]; ok {
		if data.processing {
			return false
		}
		data.inCommit = committing
	}
	return true
}

func SaveMarkerData(allocationID string, timestamp common.Timestamp, chainLength int) {
	logging.Logger.Info("SaveMarkerData", zap.Any("allocationID", allocationID), zap.Any("timestamp", timestamp), zap.Any("chainLength", chainLength))
	markerDataMut.Lock()
	defer markerDataMut.Unlock()
	var (
		data *markerData
		ok   bool
	)
	data, ok = markerDataMap[allocationID]
	if !ok {
		data = &markerData{
			firstMarkerTimestamp: timestamp,
			allocationID:         allocationID,
			chainLength:          1,
			lastMarkerTimestamp:  timestamp,
		}
		markerDataMap[allocationID] = data
	} else {
		data.chainLength = chainLength
		data.lastMarkerTimestamp = timestamp
		data.inCommit = false
		if data.chainLength == 1 {
			data.firstMarkerTimestamp = timestamp
		}
	}
	if data.processMarker() {
		logging.Logger.Info("ProcessMarkerData", zap.Any("allocationID", allocationID), zap.Any("timestamp", timestamp), zap.Any("chainLength", chainLength))
		data.processing = true
		writeMarkerChan <- data
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
	logging.Logger.Info("redeeming_write_marker", zap.String("allocationID", allocationID))
	defer func() {
		if shouldRollback {
			if rollbackErr := db.Rollback().Error; rollbackErr != nil {
				logging.Logger.Error("Error rollback on redeeming the write marker.",
					zap.Any("allocation", allocationID),
					zap.Error(rollbackErr))
			}

		} else {
			go deleteMarkerData(allocationID)
		}
	}()

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
		go deleteMarkerData(allocationID)
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

	err = wm.RedeemMarker(ctx, alloc.LastRedeemedSeq+1)
	if err != nil {
		elapsedTime := time.Since(start)
		logging.Logger.Error("Error redeeming the write marker.",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm), zap.Any("error", err), zap.Any("elapsedTime", elapsedTime))
		if retryRedeem(err.Error()) {
			go tryAgain(md)
		} else {
			go deleteMarkerData(allocationID)
		}
		shouldRollback = true
		return err
	}

	err = allocation.Repo.UpdateAllocationRedeem(ctx, allocationID, wm.WM.AllocationRoot, alloc, wm.Sequence)
	if err != nil {
		logging.Logger.Error("Error redeeming the write marker. Allocation latest wm redeemed update failed",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Any("error", err))
		shouldRollback = true
		go tryAgain(md)
		return err
	}

	err = db.Commit().Error
	if err != nil {
		logging.Logger.Error("Error committing the writemarker redeem",
			zap.Any("allocation", allocationID),
			zap.Any("wm", wm.WM.AllocationRoot), zap.Error(err))
		shouldRollback = true
		go tryAgain(md)
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
	chanSize := 400
	if len(res) > chanSize {
		chanSize = len(res)
	}
	writeMarkerChan = make(chan *markerData, chanSize)
	go startRedeemWorker(ctx)
	markerDataMut.Lock()
	defer markerDataMut.Unlock()
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
				md := &markerData{
					firstMarkerTimestamp: wm.WM.Timestamp,
					allocationID:         wm.WM.AllocationID,
					chainLength:          wm.WM.ChainLength,
					processing:           true,
					retries:              int(wm.ReedeemRetries),
				}
				markerDataMap[wm.WM.AllocationID] = md
				writeMarkerChan <- md
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
	md.retries++
	time.Sleep(time.Duration(md.retries) * 5 * time.Second)
	writeMarkerChan <- md
}

// Can add more cases where we don't want to retry
func retryRedeem(errString string) bool {
	return !strings.Contains(errString, "value not present") && !strings.Contains(errString, "Blobber is not part of the allocation")
}

func startCollector(ctx context.Context) {
	randTime := getRandTime()
	ticker := time.NewTicker(time.Duration(randTime) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			markerDataMut.Lock()
			for _, data := range markerDataMap {
				if data.processMarker() {
					logging.Logger.Info("ProcessMarkerData", zap.Any("allocationID", data.allocationID), zap.Any("timestamp", data.firstMarkerTimestamp), zap.Any("chainLength", data.chainLength))
					data.processing = true
					writeMarkerChan <- data
				}
			}
			markerDataMut.Unlock()
			randTime = getRandTime()
			ticker.Reset(time.Duration(randTime) * time.Second)
		}
	}
}

func (md *markerData) processMarker() bool {
	secondsInterval := int64(config.Configuration.MarkerRedeemInterval.Seconds())
	randTime := rand.Int63n(secondsInterval)
	randTime += secondsInterval / 2 // interval of secondsInterval/2 to 3*secondsInterval/2

	return !md.processing && !md.inCommit && (md.chainLength >= config.Configuration.MaxChainLength || common.Now()-md.firstMarkerTimestamp > common.Timestamp(config.Configuration.MaxTimestampGap) || common.Now()-md.lastMarkerTimestamp > common.Timestamp(randTime))
}

func getRandTime() int64 {
	secondsInterval := int64(config.Configuration.MarkerRedeemInterval.Seconds())
	randTime := rand.Int63n(secondsInterval / 2)
	randTime += secondsInterval / 2 // interval of secondsInterval/2 to secondsInterval
	return randTime
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
