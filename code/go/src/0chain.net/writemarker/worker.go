package writemarker

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/allocation"
	"0chain.net/common"
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

func RedeemMarkersForAllocation(allocationID string, latestWmEntity string) error {
	ctx := dbstore.WithConnection(context.Background())
	defer func() {
		err := dbstore.Commit(ctx)
		if err != nil {
			Logger.Error("Error commiting the writemarker redeem", zap.Error(err))
		}
		ctx.Done()
	}()
	allocationStatus := allocation.AllocationStatusProvider().(*allocation.AllocationStatus)
	allocationStatus.ID = allocationID
	mutex := lock.GetMutex(allocationStatus.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := allocationStatus.Read(ctx, allocationStatus.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		Logger.Error("Error in finding the allocation status from DB", zap.Error(err))
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
			if marker.DirStructure == nil {
				marker.WriteAllocationDirStructure(ctx)
			}
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

	//Logger.Info("Returning from redeem", zap.Any("wm", latestWmEntity), zap.Any("allocation", allocationID))
	return nil
}

var allocationhandler = func(ctx context.Context, key datastore.Key, value []byte) error {
	allocationObj := allocation.Provider().(*allocation.Allocation)
	err := json.Unmarshal(value, allocationObj)
	if err != nil {
		Logger.Error("Error in unmarshal of the allocation object", zap.Error(err))
		return nil
	}
	if len(allocationToProcess) > 0 && allocationObj.ID != allocationToProcess {
		return nil
	}
	allocationStatus := allocation.AllocationStatusProvider().(*allocation.AllocationStatus)
	allocationStatus.ID = allocationObj.ID
	err = allocationStatus.Read(ctx, allocationStatus.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		Logger.Error("Error in finding the allocation status from DB", zap.Error(err))
		return nil
	}
	//Logger.Info("Attempting write marker redeem", zap.Any("allocation", allocationObj), zap.Any("num_workers", numOfWorkers), zap.Any("worker_config", config.Configuration))
	if len(allocationObj.LatestWMEntity) > 0 && allocationObj.LatestWMEntity != allocationStatus.LastCommittedWMEntity {
		if numOfWorkers < config.Configuration.WMRedeemNumWorkers {
			numOfWorkers++
			redeemWorker.Add(1)
			go func(redeemCtx context.Context) {
				//Logger.Info("Starting to redeem", zap.Any("allocation", allocationObj.ID), zap.Any("wm", allocationObj.LatestWMEntity))
				err = RedeemMarkersForAllocation(allocationObj.ID, allocationObj.LatestWMEntity)
				if err != nil {
					Logger.Error("Error redeeming the write marker.", zap.Error(err))
				}
				redeemWorker.Done()
			}(context.Background())
		} else {
			allocationToProcess = allocationObj.ID
			return common.NewError("iter_break", "Breaking out of iteration")
		}

	}
	return nil
}

var redeemWorker sync.WaitGroup
var numOfWorkers = 0
var iterInprogress = false
var allocationToProcess = ""

func RedeemWriteMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.WMRedeemFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			//Logger.Info("Trying to redeem writemarkers.", zap.Any("iterInprogress", iterInprogress), zap.Any("numOfWorkers", numOfWorkers))
			if !iterInprogress && numOfWorkers == 0 {
				iterInprogress = true
				rctx := dbstore.WithReadOnlyConnection(ctx)
				err := dbstore.IteratePrefix(rctx, "allocation:", allocationhandler)
				if err == nil {
					allocationToProcess = ""
				}
				dbstore.Discard(rctx)
				rctx.Done()
				redeemWorker.Wait()
				iterInprogress = false
				numOfWorkers = 0
			}
		}
	}

}
