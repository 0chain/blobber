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
	// List allocations in a short, self-contained transaction so we don't hold
	// a transaction open across the (potentially long) per-allocation cleanup.
	var allocations []allocation.Allocation
	if err := datastore.GetStore().WithNewTransaction(func(lctx context.Context) error {
		return datastore.GetStore().GetTransaction(lctx).Find(&allocations).Error
	}); err != nil {
		return err
	}

	for _, allocationObj := range allocations {
		cleanupAllocationFiles(ctx, allocationObj)
	}
	return nil
}

// cleanupAllocationFiles deletes on-disk objects with no reference rows
// (orphans) WITHOUT starving CommitWrite, which competes for the same
// per-allocation lock. Two phases:
//
//	Phase 1 (lock-free): scan all objects, collect orphan candidates. The
//	  O(objects) scan runs without the per-allocation lock.
//	Phase 2 (brief lock per delete): take the lock, RE-CHECK the ref count is
//	  still 0 (a commit may have added a ref since phase 1), delete only if
//	  still orphaned, release. The re-check under the lock keeps it safe — a
//	  commit can't add a reference while we hold the lock.
func cleanupAllocationFiles(ctx context.Context, allocationObj allocation.Allocation) {
	logging.Logger.Info("orphan_cleanup: scanning allocation (lock-free)", zap.String("allocation_id", allocationObj.ID))

	type orphanCand struct {
		hash    string
		version int
	}
	var candidates []orphanCand

	// Phase 1 — lock-free scan in a short transaction.
	_ = datastore.GetStore().WithNewTransaction(func(sctx context.Context) error {
		sdb := datastore.GetStore().GetTransaction(sctx)
		return filestore.GetFileStore().IterateObjects(allocationObj.ID, func(hash string, contentSize int64) {
			version := 0
			h := hash
			if len(h) > 64 {
				version = 1
				h = h[:64]
			}
			var cnt int64
			if err := sdb.Table((reference.Ref{}).TableName()).
				Where(reference.Ref{ValidationRoot: h, Type: reference.FILE}).
				Or(reference.Ref{ThumbnailHash: h, Type: reference.FILE}).
				Count(&cnt).Error; err != nil {
				logging.Logger.Error("orphan_cleanup: ref count failed", zap.String("hash", h), zap.Error(err))
				return
			}
			if cnt == 0 {
				candidates = append(candidates, orphanCand{hash: h, version: version})
			}
		})
	})

	if len(candidates) == 0 {
		return
	}

	// Phase 2 — delete each candidate under a brief lock with a re-check.
	for _, c := range candidates {
		mutex := lock.GetMutex(allocationObj.TableName(), allocationObj.ID)
		mutex.Lock()
		var cnt int64
		_ = datastore.GetStore().WithNewTransaction(func(cctx context.Context) error {
			return datastore.GetStore().GetTransaction(cctx).Table((reference.Ref{}).TableName()).
				Where(reference.Ref{ValidationRoot: c.hash, Type: reference.FILE}).
				Or(reference.Ref{ThumbnailHash: c.hash, Type: reference.FILE}).
				Count(&cnt).Error
		})
		if cnt == 0 {
			if err := filestore.GetFileStore().DeleteFromFilestore(allocationObj.ID, c.hash, c.version); err != nil {
				logging.Logger.Error("FileStore_DeleteFile", zap.String("validation_root", c.hash), zap.Error(err))
			} else {
				logging.Logger.Info("orphan_cleanup: deleted orphan from disk", zap.String("hash", c.hash))
			}
		}
		mutex.Unlock()
	}
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
	db.Table((&allocation.AllocationChangeCollector{}).TableName()).Where("updated_at < ?", then).Preload("Changes").Find(&openConnectionsToDelete)

	for i := 0; i < len(openConnectionsToDelete); i++ {
		connection := &openConnectionsToDelete[i]
		processor := allocation.GetConnectionProcessor(connection.ID)
		if processor != nil {
			continue
		}
		logging.Logger.Info("Deleting temp files for the connection", zap.Any("connection", connection.ID))
		connection.ComputeProperties()

		nctx := datastore.GetStore().CreateTransaction(ctx)
		ndb := datastore.GetStore().GetTransaction(nctx)
		var errorOccurred bool
		if connection.Status == allocation.InProgressConnection || connection.Status == allocation.NewConnection {
			for _, changeProcessor := range connection.AllocationChanges {
				if err := changeProcessor.DeleteTempFile(); err != nil {
					errorOccurred = true
					logging.Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
				}
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
