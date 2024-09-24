package handler

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startCleanupTempFiles(ctx)
	go startBlackListWorker(ctx)
}

func CleanupDiskFiles(ctx context.Context) error {
	var allocations []allocation.Allocation
	db := datastore.GetStore().GetTransaction(ctx)
	db.Find(&allocations)

	for _, allocationObj := range allocations {
		cleanupAllocationFiles(ctx, allocationObj)
	}
	return nil
}

func cleanupAllocationFiles(ctx context.Context, allocationObj allocation.Allocation) {
	mutex := lock.GetMutex(allocationObj.TableName(), allocationObj.ID)
	logging.Logger.Info("cleanupAllocationLock", zap.Any("allocation_id", allocationObj.ID))
	mutex.Lock()
	defer mutex.Unlock()
	db := datastore.GetStore().GetTransaction(ctx)

	_ = filestore.GetFileStore().IterateObjects(allocationObj.ID, func(hash string, contentSize int64) {
		// thumbnail suffix makes hash greater than 65
		if len(hash) > 65 {
			return
		}
		var refs []reference.Ref
		version := 0
		if len(hash) > 64 {
			version = 1
			hash = hash[:64]
		}
		err := db.Table((reference.Ref{}).TableName()).
			Where(reference.Ref{LookupHash: hash, Type: reference.FILE}).
			Find(&refs).Error

		if err != nil {
			logging.Logger.Error("Error in cleanup of disk files.", zap.Error(err))
			return
		}

		if len(refs) == 0 {
			logging.Logger.Info("hash has no references. Deleting from disk",
				zap.Any("count", len(refs)), zap.String("hash", hash))

			if err = filestore.GetFileStore().DeleteFromFilestore(allocationObj.ID, hash, version); err != nil {
				logging.Logger.Error("FileStore_DeleteFile", zap.String("validation_root", hash), zap.Error(err))
			}
		}
	})
}

func cleanupTempFiles(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] cleanupTempFiles", zap.Any("err", r))
		}
	}()

	rctx := datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(rctx)
	now := time.Now().UTC()
	then := now.Add(time.Duration(-config.Configuration.OpenConnectionWorkerTolerance) * time.Second)

	var openConnectionsToDelete []allocation.AllocationChangeCollector
	db.Table((&allocation.AllocationChangeCollector{}).TableName()).Where("updated_at < ? AND status IN (?,?)", then, allocation.NewConnection, allocation.InProgressConnection).Preload("Changes").Find(&openConnectionsToDelete)

	for i := 0; i < len(openConnectionsToDelete); i++ {
		connection := &openConnectionsToDelete[i]
		logging.Logger.Info("Deleting temp files for the connection", zap.Any("connection", connection.ID))
		processor := allocation.GetConnectionProcessor(connection.ID)
		if processor != nil {
			continue
		}
		connection.ComputeProperties()

		nctx := datastore.GetStore().CreateTransaction(ctx)
		ndb := datastore.GetStore().GetTransaction(nctx)
		var errorOccurred bool
		for _, changeProcessor := range connection.AllocationChanges {
			if err := changeProcessor.DeleteTempFile(); err != nil {
				errorOccurred = true
				logging.Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
			}
		}

		if !errorOccurred {
			for _, c := range connection.Changes {
				ndb.Unscoped().Delete(c)
			}
			ndb.Unscoped().Delete(connection)
			allocation.DeleteConnectionObjEntry(connection.ID)
		}

		ndb.Commit()
		nctx.Done()
	}

	db.Commit()
	rctx.Done()
}

func startCleanupTempFiles(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.OpenConnectionWorkerFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanupTempFiles(ctx)
		}
	}
}
