package handler

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
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
			Logger.Error("Error in cleanup of disk files.", zap.Error(err))
			return
		}
		if len(refs) == 0 {
			Logger.Info("hash has no references. Deleting from disk", zap.Any("count", len(refs)), zap.String("hash", contentHash))
			if err := filestore.GetFileStore().DeleteFile(allocationObj.ID, contentHash); err != nil {
				Logger.Error("FileStore_DeleteFile", zap.String("content_hash", contentHash), zap.Error(err))
			}
		}
	})

}

func cleanupTempFiles(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("[recover] cleanupTempFiles", zap.Any("err", r))
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
		Logger.Info("Deleting temp files for the connection", zap.Any("connection", connection.ID))
		connection.ComputeProperties()
		nctx := datastore.GetStore().CreateTransaction(ctx)
		ndb := datastore.GetStore().GetTransaction(nctx)
		for _, changeProcessor := range connection.AllocationChanges {
			if err := changeProcessor.DeleteTempFile(); err != nil {
				Logger.Error("AllocationChangeProcessor_DeleteTempFile", zap.Error(err))
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
	var iterInprogress = false
	ticker := time.NewTicker(time.Duration(config.Configuration.OpenConnectionWorkerFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			//Logger.Info("Trying to redeem writemarkers.", zap.Any("iterInprogress", iterInprogress), zap.Any("numOfWorkers", numOfWorkers))
			if !iterInprogress {
				iterInprogress = true //nolint:ineffassign // probably has something to do with goroutines
				cleanupTempFiles(ctx)
				iterInprogress = false
			}
		}
	}
}

func moveColdDataToCloud(ctx context.Context, coldStorageMinFileSize, limit int64) {
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("[recover] moveColdDataToCloud", zap.Any("err", r))
		}
	}()

	fs := filestore.GetFileStore()
	totalDiskSizeUsed, err := fs.GetTotalDiskSizeUsed()
	if err != nil {
		Logger.Error("Unable to get total disk size used from the file store", zap.Error(err))
		return
	}

	// Check if capacity exceded the start capacity size
	if totalDiskSizeUsed > config.Configuration.ColdStorageStartCapacitySize {
		rctx := datastore.GetStore().CreateTransaction(ctx)
		db := datastore.GetStore().GetTransaction(rctx)
		// Get total number of fileRefs with size greater than limit and on_cloud = false
		var totalRecords int64
		db.Table((&reference.Ref{}).TableName()).
			Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).
			Count(&totalRecords)

		offset := int64(0)
		for offset < totalRecords {
			// Get all fileRefs with size greater than limit and on_cloud false
			var fileRefs []*reference.Ref
			db.Offset(int(offset)).Limit(int(limit)).
				Table((&reference.Ref{}).TableName()).
				Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).
				Find(&fileRefs)

			for _, fileRef := range fileRefs {
				if fileRef.Type == reference.DIRECTORY {
					continue
				}

				fileStat, err := stats.GetFileStats(rctx, fileRef.ID)
				if err != nil {
					Logger.Error("Unable to find filestats for fileRef with", zap.Any("reID", fileRef.ID))
					continue
				}

				timeToAdd := time.Duration(config.Configuration.ColdStorageTimeLimitInHours) * time.Hour
				if fileStat.UpdatedAt.Before(time.Now().Add(-1 * timeToAdd)) {
					Logger.Info("Moving file to cloud", zap.Any("path", fileRef.Path), zap.Any("allocation", fileRef.AllocationID))
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
	var iterInprogress = false
	var coldStorageMinFileSize = config.Configuration.ColdStorageMinimumFileSize
	var limit = config.Configuration.ColdStorageJobQueryLimit
	ticker := time.NewTicker(time.Duration(config.Configuration.MinioWorkerFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress {
				moveColdDataToCloud(ctx, coldStorageMinFileSize, limit)
				iterInprogress = false
				stats.LastMinioScan = time.Now()
				Logger.Info("Move cold data to cloud worker running successfully")
			}
		}
	}
}

func moveFileToCloud(ctx context.Context, fileRef *reference.Ref) {
	fs := filestore.GetFileStore()
	allocation, err := fs.SetupAllocation(fileRef.AllocationID, true)
	if err != nil {
		Logger.Error("Unable to fetch allocation with error", zap.Any("allocationID", fileRef.AllocationID), zap.Error(err))
		return
	}

	dirPath, destFile := filestore.GetFilePathFromHash(fileRef.ContentHash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

	err = fs.UploadToCloud(fileRef.ContentHash, fileObjectPath)
	if err != nil {
		Logger.Error("Error uploading cold data to cloud", zap.Error(err), zap.Any("file_name", fileRef.Name), zap.Any("file_path", fileObjectPath))
		return
	}

	fileRef.OnCloud = true
	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)
	err = db.Save(fileRef).Error
	if err != nil {
		Logger.Error("Failed to update reference_object for on cloud true", zap.Error(err))
		db.Rollback()
		ctx.Done()
		return
	}

	db.Commit()
	ctx.Done()
	Logger.Info("Successfully uploaded file to cloud", zap.Any("file_name", fileRef.Name), zap.Any("allocation", fileRef.AllocationID))

	if config.Configuration.ColdStorageDeleteLocalCopy {
		err = os.Remove(fileObjectPath)
		if err != nil {
			Logger.Error("Error deleting file after upload to cold storage", zap.Error(err))
			return
		}
		Logger.Info("Successfully deleted file's local copy", zap.Any("file_name", fileRef.Name), zap.Any("allocation", fileRef.AllocationID))
	}
}
