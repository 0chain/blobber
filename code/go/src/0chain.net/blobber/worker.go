package blobber

import (
	"container/list"
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/allocation"
	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/lock"
	. "0chain.net/logging"
	"0chain.net/readmarker"
	"0chain.net/reference"
	"0chain.net/transaction"
	"0chain.net/writemarker"
	"go.uber.org/zap"
)

//SetupWorkers - setup workers */
func SetupWorkers(ctx context.Context) {
	go RedeemMarkers(ctx)
	go CleanupOpenConnections(ctx)
	go CleanupContentRef(ctx)
}

func CleanupContentRef(ctx context.Context) {
	ticker := time.NewTicker(3600 * time.Second)

	dbstore := GetMetaDataStore()
	contentRefHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		contentRef := reference.ContentReferenceProvider().(*reference.ContentReference)
		err := json.Unmarshal(value, contentRef)
		if err != nil {
			return err
		}
		if contentRef.ReferenceCount > 0 || common.Within(int64(contentRef.LastUpdated), 3600) {
			return nil
		}
		Logger.Info("Removing file content with no activity in the last hour and no more references", zap.Any("contentref", contentRef))

		err = contentRef.Delete(ctx)
		if err != nil {
			Logger.Error("Error deleting the content ref", zap.Error(err))
		}
		err = fileStore.DeleteFile(contentRef.AllocationID, contentRef.ContentHash)
		if err != nil {
			Logger.Error("Error deleting the content for the contentref", zap.Error(err))
		}
		return nil
	}
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ctx = dbstore.WithConnection(ctx)
			err := dbstore.IteratePrefix(ctx, "contentref:", contentRefHandler)
			if err != nil {
				dbstore.Discard(ctx)
			} else {
				dbstore.Commit(ctx)
			}
		}
	}
}

func RedeemReadMarker(ctx context.Context, rmEntity *readmarker.ReadMarkerEntity) error {

	//&& (rmEntity.LastestRedeemedRM == nil || rmEntity.LastestRedeemedRM.ReadCounter < rmEntity.LatestRM.ReadCounter)
	rmStatus := &readmarker.ReadMarkerStatus{}
	rmStatus.LastestRedeemedRM = &readmarker.ReadMarker{ClientID: rmEntity.LatestRM.ClientID, BlobberID: rmEntity.LatestRM.BlobberID}
	mutex := lock.GetMutex(rmStatus.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := rmStatus.Read(ctx, rmStatus.GetKey())

	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}

	if (err != nil && err == datastore.ErrKeyNotFound) || (err == nil && rmStatus.LastestRedeemedRM.ReadCounter < rmEntity.LatestRM.ReadCounter) {
		Logger.Info("Redeeming the read marker", zap.Any("rm", rmEntity.LatestRM))
		params := make(map[string]string)
		params["blobber"] = rmEntity.LatestRM.BlobberID
		params["client"] = rmEntity.LatestRM.ClientID
		var latestRM readmarker.ReadMarker
		_, errsc := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params, chain.GetServerChain(), &latestRM)
		if errsc == nil {
			Logger.Info("Latest read marker from blockchain", zap.Any("rm", latestRM))
			if latestRM.ReadCounter > 0 && latestRM.ReadCounter >= rmEntity.LatestRM.ReadCounter {
				Logger.Info("Updating the local state to match the block chain")
				rmStatus.LastestRedeemedRM = rmEntity.LatestRM
				rmStatus.LastRedeemTxnID = "sync"
				rmStatus.Write(ctx)
				return nil
			}
		} else {
			Logger.Error("Error from sc rest api call", zap.Error(errsc))
		}
		err := GetProtocolImpl(rmEntity.LatestRM.AllocationID).RedeemReadMarker(ctx, rmEntity.LatestRM, rmStatus)
		if err != nil {
			Logger.Error("Error redeeming the read marker.", zap.Any("rm", rmEntity), zap.Any("error", err))
			return err
		}
		Logger.Info("Successfully redeemed read marker", zap.Any("rm", rmEntity.LatestRM), zap.Any("rm_status", rmStatus))
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
			Logger.Info("Redeeming the write marker", zap.Any("wm", marker.GetKey()))
			wmMutex := lock.GetMutex(marker.GetKey())
			wmMutex.Lock()
			err := GetProtocolImpl(marker.WM.AllocationID).RedeemMarker(ctx, marker)
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

func CleanupOpenConnections(ctx context.Context) {
	ticker := time.NewTicker(3600 * time.Second)

	dbstore := GetMetaDataStore()
	allocationChangeHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
		err := json.Unmarshal(value, connectionObj)
		if err != nil {
			return err
		}
		if common.Within(int64(connectionObj.LastUpdated), 3600) {
			return nil
		}
		Logger.Info("Removing open connection with no activity in the last hour", zap.Any("connection", connectionObj))

		err = connectionObj.DeleteChanges(ctx, fileStore)
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
	doneChanMapMutex := sync.RWMutex{}
	dbstore := GetMetaDataStore()

	// allhandler := func(ctx context.Context, key datastore.Key, value []byte) error {
	// 	fmt.Println(string(key))
	// 	fmt.Println(string(value))
	// 	return nil
	// }

	allocationhandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		allocationObj := allocation.Provider().(*allocation.Allocation)
		err := json.Unmarshal(value, allocationObj)
		if err != nil {
			return err
		}
		doneChanMapMutex.RLock()
		inprogress, ok := doneChanMap[allocationObj.ID]
		doneChanMapMutex.RUnlock()
		if len(allocationObj.LatestWMEntity) > 0 && (!ok || !inprogress) {
			go func(redeemCtx context.Context) {
				doneChanMapMutex.Lock()
				doneChanMap[allocationObj.ID] = true
				doneChanMapMutex.Unlock()
				redeemCtx = dbstore.WithConnection(redeemCtx)
				err = RedeemMarkersForAllocation(redeemCtx, allocationObj.ID, allocationObj.LatestWMEntity)
				if err != nil {
					Logger.Error("Error redeeming the write marker.", zap.Error(err))
				}
				dbstore.Commit(redeemCtx)
				doneChanMapMutex.Lock()
				delete(doneChanMap, allocationObj.ID)
				doneChanMapMutex.Unlock()
			}(context.WithValue(ctx, "write_marker_redeem", "true"))

		}
		return nil
	}

	rmHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		rmEntity := readmarker.Provider().(*readmarker.ReadMarkerEntity)
		err := json.Unmarshal(value, rmEntity)
		if err != nil {
			return err
		}
		doneChanMapMutex.RLock()
		inprogress, ok := doneChanMap[rmEntity.GetKey()]
		doneChanMapMutex.RUnlock()
		if rmEntity.LatestRM != nil && (!ok || !inprogress) {
			go func(redeemCtx context.Context) {
				doneChanMapMutex.Lock()
				doneChanMap[rmEntity.GetKey()] = true
				doneChanMapMutex.Unlock()
				redeemCtx = dbstore.WithConnection(redeemCtx)
				err := RedeemReadMarker(redeemCtx, rmEntity)
				if err != nil {
					Logger.Error("Error redeeming the read marker.", zap.Error(err))
				}
				dbstore.Commit(redeemCtx)
				doneChanMapMutex.Lock()
				delete(doneChanMap, rmEntity.GetKey())
				doneChanMapMutex.Unlock()
			}(context.WithValue(ctx, "read_marker_redeem", "true"))
		}
		return nil
	}

	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			//dbstore.Iterate(ctx, allhandler)
			dbstore.IteratePrefix(ctx, "allocation:", allocationhandler)
			dbstore.IteratePrefix(ctx, "rm:", rmHandler)
		}
	}

}
