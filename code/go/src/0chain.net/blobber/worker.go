package blobber

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"0chain.net/allocation"
	"0chain.net/common"
	"0chain.net/datastore"
	. "0chain.net/logging"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

//SetupWorkers - setup workers */
func SetupWorkers(ctx context.Context) {
	go RedeemMarkers(ctx)
	go CleanupOpenConnections(ctx)
}

func RedeemMarkersForAllocation(ctx context.Context, allocationID string, latestWmEntity string) {

	allocationStatus := allocation.AllocationStatusProvider().(*allocation.AllocationStatus)
	allocationStatus.ID = allocationID
	mutex := GetMutex(allocationStatus.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := allocationStatus.Read(ctx, allocationStatus.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return
	}
	currWmEntity := latestWmEntity

	//unredeemedMarkers := make([]*writemarker.WriteMarkerEntity, 0)
	unredeemedMarkers := list.New()
	for currWmEntity != allocationStatus.LastCommittedWMEntity {
		//unredeemedMarkers = append(unredeemedMarkers, latestWM)
		wmEntity := writemarker.Provider().(*writemarker.WriteMarkerEntity)
		wmEntity.Read(ctx, currWmEntity)
		unredeemedMarkers.PushFront(wmEntity)
		currWmEntity = wmEntity.PrevWM
	}

	for e := unredeemedMarkers.Front(); e != nil; e = e.Next() {
		marker := e.Value.(*writemarker.WriteMarkerEntity)
		if marker.Status != writemarker.Committed {
			err := GetProtocolImpl(marker.WM.AllocationID).RedeemMarker(ctx, marker)
			if err != nil {
				Logger.Error("Error redeeming the write marker.", zap.Any("wm", marker), zap.Any("error", err))
				continue
			}
			allocationStatus.LastCommittedWMEntity = marker.GetKey()
			allocationStatus.Write(ctx)
		}
	}
}

func CleanupOpenConnections(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)

	dbstore := GetMetaDataStore()
	allocationChangeHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		connectionObj := AllocationChangeCollectorProvider().(*AllocationChangeCollector)
		err := json.Unmarshal(value, connectionObj)
		if err != nil {
			return err
		}
		if common.Within(int64(connectionObj.LastUpdated), 3600) {
			return nil
		}
		Logger.Info("Removing open connection with no activity in the last hour", zap.Any("connection", connectionObj))

		err = connectionObj.DeleteChanges(ctx)
		return err
	}
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ctx = dbstore.WithConnection(ctx)
			err := dbstore.IteratePrefix(ctx, "allocation_change:", allocationChangeHandler)
			if err != nil {
				dbstore.Discard(ctx)
			} else {
				dbstore.Commit(ctx)
			}
		}
	}
}

/*CleanupWorker - a worker to delete transactiosn that are no longer valid */
func RedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	doneChanMap := make(map[string]bool)
	dbstore := GetMetaDataStore()

	allhandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		fmt.Println(string(key))
		fmt.Println(string(value))
		return nil
	}

	allocationhandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		allocationObj := allocation.Provider().(*allocation.Allocation)
		err := json.Unmarshal(value, allocationObj)
		if err != nil {
			return err
		}
		inprogress, ok := doneChanMap[allocationObj.ID]
		if len(allocationObj.LatestWMEntity) > 0 && (!ok || !inprogress) {
			go func() {
				doneChanMap[allocationObj.ID] = true
				ctx = dbstore.WithConnection(ctx)
				RedeemMarkersForAllocation(ctx, allocationObj.ID, allocationObj.LatestWMEntity)
				dbstore.Commit(ctx)
				doneChanMap[allocationObj.ID] = false
			}()

		}
		return nil
	}

	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dbstore.Iterate(ctx, allhandler)
			dbstore.IteratePrefix(ctx, "allocation:", allocationhandler)
		}
	}

}
