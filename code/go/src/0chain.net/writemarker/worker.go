package writemarker

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/allocation"
	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
	. "0chain.net/logging"

	"go.uber.org/zap"
)

var dbstore datastore.Store
var fileStore filestore.FileStore

func SetupWorkers(ctx context.Context, metaStore datastore.Store, fsStore filestore.FileStore) {
	dbstore = metaStore
	fileStore = fsStore
	go RedeemWriteMarkers(ctx)
}

var allocationhandler = func(ctx context.Context, key datastore.Key, value []byte) error {
	allocationObj := allocation.Provider().(*allocation.Allocation)
	err := json.Unmarshal(value, allocationObj)
	if err != nil {
		return err
	}
	if len(allocationObj.LatestWMEntity) > 0 && numOfWorkers < config.Configuration.WMRedeemNumWorkers {
		numOfWorkers++
		redeemWorker.Add(1)
		go func(redeemCtx context.Context) {
			redeemCtx = dbstore.WithConnection(redeemCtx)
			err = RedeemMarkersForAllocation(redeemCtx, allocationObj.ID, allocationObj.LatestWMEntity)
			if err != nil {
				Logger.Error("Error redeeming the write marker.", zap.Error(err))
			}
			dbstore.Commit(redeemCtx)
			redeemWorker.Done()
		}(context.WithValue(ctx, "write_marker_redeem", "true"))

	}
	return nil
}

func RedeemMarkersForAllocation(ctx context.Context, allocationID string, latestWmEntity string) error {

	allocationStatus := allocation.AllocationStatusProvider().(*allocation.AllocationStatus)
	allocationStatus.ID = allocationID
	mutex := lock.GetMutex(allocationStatus.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := allocationStatus.Read(ctx, allocationStatus.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	currWmEntity := latestWmEntity

	//unredeemedMarkers := make([]*WriteMarkerEntity, 0)
	unredeemedMarkers := list.New()
	for currWmEntity != allocationStatus.LastCommittedWMEntity {
		//unredeemedMarkers = append(unredeemedMarkers, latestWM)
		wmEntity := Provider().(*WriteMarkerEntity)
		wmEntity.Read(ctx, currWmEntity)
		unredeemedMarkers.PushFront(wmEntity)
		currWmEntity = wmEntity.PrevWM
	}

	for e := unredeemedMarkers.Front(); e != nil; e = e.Next() {
		marker := e.Value.(*WriteMarkerEntity)
		if marker.Status != Committed {
			Logger.Info("Redeeming the write marker", zap.Any("wm", marker.GetKey()))
			wmMutex := lock.GetMutex(marker.GetKey())
			wmMutex.Lock()
			err := marker.RedeemMarker(ctx)
			if err != nil {
				Logger.Error("Error redeeming the write marker.", zap.Any("wm", marker.GetKey()), zap.Any("error", err))
				wmMutex.Unlock()
				continue
			}
			marker.WriteAllocationDirStructure(ctx)
			marker.Write(ctx)
			wmMutex.Unlock()

			allocationStatus.LastCommittedWMEntity = marker.GetKey()
			err = allocationStatus.Write(ctx)
			if err != nil {
				Logger.Error("Error redeeming the write marker. Allocation status update failed", zap.Any("wm", marker.GetKey()), zap.Any("error", err))
				return err
			}
			Logger.Info("Success Redeeming the write marker", zap.Any("wm", marker.GetKey()), zap.Any("txn", marker.CloseTxnID))
		}
	}
	return nil
}

var redeemWorker sync.WaitGroup
var numOfWorkers = 0
var iterInprogress = false

func RedeemWriteMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.WMRedeemFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress && numOfWorkers == 0 {
				iterInprogress = true
				dbstore.IteratePrefix(ctx, "allocation:", allocationhandler)
				redeemWorker.Wait()
				iterInprogress = false
				numOfWorkers = 0
			}
		}
	}

}
