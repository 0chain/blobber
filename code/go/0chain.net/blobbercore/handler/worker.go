package handler

import (
	"0chain.net/core/lock"
	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/filestore"
	"context"
	"time"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go CleanupTempFiles(ctx)
}


func CleanupDiskFiles(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	var allocations []allocation.Allocation
	db.Find(&allocations)
	for _, allocationObj := range allocations {
		mutex := lock.GetMutex(allocationObj.TableName(), allocationObj.ID)
		mutex.Lock()
		filestore.GetFileStore().IterateObjects(allocationObj.ID, func(contentHash string, contentSize int64) {
			var refs []reference.Ref
			db.Table((reference.Ref{}).TableName()).Where(reference.Ref{ContentHash: contentHash, Type: reference.FILE}).Or(reference.Ref{ThumbnailHash: contentHash, Type: reference.FILE}).Find(&refs)
			Logger.Info("hash has a reference", zap.Any("count", len(refs)), zap.String("hash", contentHash))
			// if count == 0 {
			// 	filestore.GetFileStore().DeleteFile(allocationObj.ID, contentHash)
			// }
		})
		mutex.Unlock()
	}
	return nil
}


var iterInprogress = false

func CleanupTempFiles(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.OpenConnectionWorkerFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			//Logger.Info("Trying to redeem writemarkers.", zap.Any("iterInprogress", iterInprogress), zap.Any("numOfWorkers", numOfWorkers))
			if !iterInprogress {
				iterInprogress = true
				rctx := datastore.GetStore().CreateTransaction(ctx)
				db := datastore.GetStore().GetTransaction(rctx)
				now := time.Now()
				then := now.Add(time.Duration(-config.Configuration.OpenConnectionWorkerTolerance) * time.Second)
				var openConnectionsToDelete []allocation.AllocationChangeCollector
				db.Table((&allocation.AllocationChangeCollector{}).TableName()).Where("updated_at < ? AND status IN (?,?)", then, allocation.NewConnection, allocation.InProgressConnection).Preload("Changes").Find(&openConnectionsToDelete)
				for _, connection := range openConnectionsToDelete {
					Logger.Info("Deleting temp files for the connection", zap.Any("connection", connection.ConnectionID))
					connection.ComputeProperties()
					nctx := datastore.GetStore().CreateTransaction(ctx)
					ndb := datastore.GetStore().GetTransaction(nctx)
					for _, changeProcessor := range connection.AllocationChanges {
						changeProcessor.DeleteTempFile()
					}
					ndb.Model(connection).Updates(allocation.AllocationChangeCollector{Status: allocation.DeletedConnection})
					ndb.Commit()
					nctx.Done()
				}
				db.Rollback()
				rctx.Done()
				iterInprogress = false
			}
		}
	}
}
