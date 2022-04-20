package handler

import (
	"context"
	"os"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startCleanupTempFiles(ctx)
	if config.Configuration.MinioStart {
		go startMoveColdDataToCloud(ctx)
	}
}

func CleanupDiskFiles(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	var allocations []allocation.Allocation
	db.Find(&allocations)
	for _, allocationObj := range allocations {
		cleanupAllocationFiles(db, &allocationObj)
	}
	return nil
}

func cleanupAllocationFiles(db *gorm.DB, allocationObj *allocation.Allocation) {
	mutex := lock.GetMutex(allocationObj.TableName(), allocationObj.ID)
	mutex.Lock()
	defer mutex.Unlock()
	_ = filestore.GetFileStore().IterateObjects(allocationObj.ID, func(contentHash string, contentSize int64) {
		var refs []reference.Ref
		err := db.Table((reference.Ref{}).TableName()).Where(reference.Ref{ContentHash: contentHash, Type: reference.FILE}).Or(reference.Ref{ThumbnailHash: contentHash, Type: reference.FILE}).Find(&refs).Error
		if err != nil {
			logging.Logger.Error("Error in cleanup of disk files.", zap.Error(err))
			return
		}
		if len(refs) == 0 {
			logging.Logger.Info("hash has no references. Deleting from disk", zap.Any("count", len(refs)), zap.String("hash", contentHash))
			if err := filestore.GetFileStore().DeleteFile(allocationObj.ID, contentHash); err != nil {
				logging.Logger.Error("FileStore_DeleteFile", zap.String("content_hash", contentHash), zap.Error(err))
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
		connection.ComputeProperties()
		nctx := datastore.GetStore().CreateTransaction(ctx)
		ndb := datastore.GetStore().GetTransaction(nctx)
		for _, changeProcessor := range connection.AllocationChanges {
			if err := changeProcessor.DeleteTempFile(); err != nil {
				logging.Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
			}
		}
		ndb.Model(connection).Updates(allocation.AllocationChangeCollector{Status: allocation.DeletedConnection})
		ndb.Commit()
		nctx.Done()
	}
	db.Rollback()
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

func moveColdDataToCloud(ctx context.Context, coldStorageMinFileSize int64, limit int) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] moveColdDataToCloud", zap.Any("err", r))
		}
	}()

	fs := filestore.GetFileStore()
	totalDiskSizeUsed, err := fs.GetTotalDiskSizeUsed()
	if err != nil {
		logging.Logger.Error("Unable to get total disk size used from the file store", zap.Error(err))
		return
	}

	// Check if capacity exceded the start capacity size
	if totalDiskSizeUsed > config.Configuration.ColdStorageStartCapacitySize {
		rctx := datastore.GetStore().CreateTransaction(ctx)
		db := datastore.GetStore().GetTransaction(rctx)
		// Get total number of fileRefs with size greater than limit and on_cloud = false
		var totalRecords int64
		db.Model(&reference.Ref{}).
			Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).
			Count(&totalRecords)

		var offset int
		for int64(offset) < totalRecords {
			var fileRefs []*reference.Ref
			db.Offset(offset).Limit(limit).
				Table((&reference.Ref{}).TableName()).
				Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).
				Find(&fileRefs)

			for _, fileRef := range fileRefs {
				if fileRef.Type == reference.DIRECTORY {
					continue
				}

				fileStat, err := stats.GetFileStats(rctx, fileRef.ID)
				if err != nil {
					logging.Logger.Error("Unable to find filestats for fileRef with", zap.Any("reID", fileRef.ID))
					continue
				}

				timeToAdd := time.Duration(config.Configuration.ColdStorageTimeLimitInHours) * time.Hour
				if fileStat.UpdatedAt.Before(time.Now().Add(-1 * timeToAdd)) {
					logging.Logger.Info("Moving file to cloud", zap.Any("path", fileRef.Path), zap.Any("allocation", fileRef.AllocationID))
					moveFileToCloud(ctx, fileRef)
				}
			}
			offset += limit
		}
		db.Commit()
		rctx.Done()
	}
}

func startMoveColdDataToCloud(ctx context.Context) {
	var coldStorageMinFileSize = config.Configuration.ColdStorageMinimumFileSize
	var limit = config.Configuration.ColdStorageJobQueryLimit
	ticker := time.NewTicker(time.Duration(config.Configuration.MinioWorkerFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			moveColdDataToCloud(ctx, coldStorageMinFileSize, limit)
			stats.LastMinioScan = time.Now()
			logging.Logger.Info("Move cold data to cloud worker running successfully")
		}
	}
}

func moveFileToCloud(ctx context.Context, fileRef *reference.Ref) {
	fileObjectPath, err := filestore.GetPathForFile(fileRef.AllocationID, fileRef.ContentHash)
	if err != nil {
		logging.Logger.Error("Error while getting path of file", zap.Error(err))
	}
	fs := filestore.GetFileStore()
	err = fs.UploadToCloud(fileRef.ContentHash, fileObjectPath)
	if err != nil {
		logging.Logger.Error("Error uploading cold data to cloud", zap.Error(err), zap.Any("file_name", fileRef.Name), zap.Any("file_path", fileObjectPath))
		return
	}

	if fileRef.ThumbnailHash != "" {
		thumbnailPath := ""
		if err := fs.UploadToCloud(fileRef.ThumbnailHash, thumbnailPath); err != nil {
			logging.Logger.Error("Error uploading cold thumbnail data to cloud", zap.Error(err))

			logging.Logger.Info("Removing file from cloud")
			if err := fs.RemoveFromCloud(fileRef.ContentHash); err != nil {
				logging.Logger.Debug("Got Error while remove file from cloud", zap.String("file", fileRef.ContentHash))
			}
			return
		}
	}

	fileRef.OnCloud = true
	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)
	err = db.Save(fileRef).Error
	if err != nil {
		logging.Logger.Error("Failed to update reference_object for on cloud true", zap.Error(err))
		db.Rollback()
		ctx.Done()
		return
	}

	db.Commit()
	ctx.Done()
	logging.Logger.Info("Successfully uploaded file to cloud", zap.Any("file_name", fileRef.Name), zap.Any("allocation", fileRef.AllocationID))

	newCtx := datastore.GetStore().CreateTransaction(context.Background())
	db = datastore.GetStore().GetTransaction(newCtx)

	var contentHashSharingRefsCount int64
	condRef := reference.Ref{AllocationID: fileRef.AllocationID, ContentHash: fileRef.ContentHash}
	if err := db.Model(&reference.Ref{}).Where(&condRef).Count(&contentHashSharingRefsCount).Error; err != nil {
		logging.Logger.Error("Failed to get count", zap.Error(err))
		return
	}

	if contentHashSharingRefsCount < 2 && config.Configuration.ColdStorageDeleteLocalCopy {
		err = os.Remove(fileObjectPath)
		if err != nil {
			logging.Logger.Error("Error deleting file after upload to cold storage", zap.Error(err))
			return
		}
		logging.Logger.Info("Successfully deleted file's local copy", zap.Any("file_name", fileRef.Name), zap.Any("allocation", fileRef.AllocationID))
	}
}
