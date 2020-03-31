package handler

import (
	"context"
	"path/filepath"
	"time"

	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/stats"
	"0chain.net/core/lock"
	"github.com/jinzhu/gorm"
	"github.com/remeh/sizedwaitgroup"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	. "0chain.net/core/logging"
	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go CleanupTempFiles(ctx)
	go MoveColdDataToCloud(ctx)
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
			err := db.Table((reference.Ref{}).TableName()).Where(reference.Ref{ContentHash: contentHash, Type: reference.FILE}).Or(reference.Ref{ThumbnailHash: contentHash, Type: reference.FILE}).Find(&refs).Error
			if err != nil && !gorm.IsRecordNotFoundError(err) {
				Logger.Error("Error in cleanup of disk files.", zap.Error(err))
				return
			}
			if len(refs) == 0 {
				Logger.Info("hash has no references. Deleting from disk", zap.Any("count", len(refs)), zap.String("hash", contentHash))
				filestore.GetFileStore().DeleteFile(allocationObj.ID, contentHash)
			}
			return
		})
		mutex.Unlock()
	}
	return nil
}

func CleanupTempFiles(ctx context.Context) {
	var iterInprogress = false
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

func MoveColdDataToCloud(ctx context.Context) {
	var iterInprogress = false
	var coldStorageMinFileSize = config.Configuration.ColdStorageMinimumFileSize
	var limit = config.Configuration.ColdStorageJobQueryLimit
	ticker := time.NewTicker(time.Duration(config.Configuration.MinioWorkerFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress {
				iterInprogress = true
				rctx := datastore.GetStore().CreateTransaction(ctx)
				db := datastore.GetStore().GetTransaction(rctx)
				fs := filestore.GetFileStore()

				// Get total number of fileRefs with size greater than limit and on_cloud = false
				var totalRecords int64
				db.Table((&reference.Ref{}).TableName()).Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).Count(&totalRecords)
				offset := int64(0)

				for offset < totalRecords {
					// Get all fileRefs with size greater than limit and on_cloud false
					var fileRefs []reference.Ref
					db.Offset(offset).Limit(limit).
						Table((&reference.Ref{}).TableName()).
						Where("size > ? AND on_cloud = ?", coldStorageMinFileSize, false).
						Find(&fileRefs)

					// Create sized wait group to do concurrent uploads
					swg := sizedwaitgroup.New(config.Configuration.MinioNumWorkers)
					for _, fileRef := range fileRefs {
						// Get file stats for the given fileRef
						fileStat, err := stats.GetFileStats(ctx, fileRef.ID)
						if err != nil {
							Logger.Error("Unable to find filestats for fileRef with", zap.Any("reID", fileRef.ID))
							continue
						}

						// Check if last updatedAt is olde then than the given limit
						timeToAdd := time.Duration(config.Configuration.ColdStorageTimeLimitInHours) * time.Hour
						if fileStat.UpdatedAt.Before(time.Now().Add(-1 * timeToAdd)) {

							// Setup allocation for the filrRef
							allocation, err := fs.SetupAllocation(fileRef.AllocationID, true)
							if err != nil {
								Logger.Error("Unable to fetch allocation with error", zap.Any("allocationID", fileRef.AllocationID), zap.Error(err))
								continue
							}

							// Parse file object path
							dirPath, destFile := filestore.GetFilePathFromHash(fileRef.ContentHash)
							fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
							fileObjectPath = filepath.Join(fileObjectPath, destFile)

							// Process upload to cloud
							swg.Add()
							go func(fs filestore.FileStore, fileRef reference.Ref, filePath string) {
								defer swg.Done()
								_, err := fs.UploadToCloud(fileRef.Name, filePath)
								if err != nil {
									Logger.Error("Error uploading cold data to cloud", zap.Error(err), zap.Any("file_name", fileRef.Name), zap.Any("file_path", filePath))
								} else {
									// Update fileRef with on cloud true
									db.Table((&reference.Ref{}).TableName()).
										Where(&reference.Ref{ID: fileRef.ID}).
										Update("on_cloud", true)

									// Delete file from blobber
									err = fs.DeleteFile(fileRef.AllocationID, fileRef.ContentHash)
									if err != nil {
										Logger.Error("Error deleting file after upload to cold storage", zap.Error(err))
									}
								}
							}(fs, fileRef, fileObjectPath)
						}
					}
					swg.Wait()
					offset = offset + limit
				}
				db.Rollback()
				rctx.Done()
				iterInprogress = false
				Logger.Info("Move cold data to cloud worker running successfully")
			}
		}
	}
}
