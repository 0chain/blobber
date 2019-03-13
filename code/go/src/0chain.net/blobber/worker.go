package blobber

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/allocation"
	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/datastore"
	. "0chain.net/logging"
	"0chain.net/reference"
	"go.uber.org/zap"
)

//SetupWorkers - setup workers */
func SetupWorkers(ctx context.Context) {
	go CleanupOpenConnections(ctx)
	go CleanupContentRef(ctx)
}

func CleanupContentRef(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.ContentRefWorkerFreq) * time.Second)

	dbstore := GetMetaDataStore()
	contentRefHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		contentRef := reference.ContentReferenceProvider().(*reference.ContentReference)
		err := json.Unmarshal(value, contentRef)
		if err != nil {
			return err
		}
		if contentRef.ReferenceCount > 0 || common.Within(int64(contentRef.LastUpdated), config.Configuration.ContentRefWorkerTolerance) {
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

func CleanupOpenConnections(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.OpenConnectionWorkerFreq) * time.Second)

	dbstore := GetMetaDataStore()
	allocationChangeHandler := func(ctx context.Context, key datastore.Key, value []byte) error {
		connectionObj := allocation.AllocationChangeCollectorProvider().(*allocation.AllocationChangeCollector)
		err := json.Unmarshal(value, connectionObj)
		if err != nil {
			return err
		}
		if common.Within(int64(connectionObj.LastUpdated), config.Configuration.OpenConnectionWorkerTolerance) {
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
