package blobber

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/writemarker"

	"0chain.net/datastore"

	"0chain.net/badgerdbstore"
)

//SetupWorkers - setup workers */
func SetupWorkers(ctx context.Context) {
	go RedeemMarkers(ctx)
}

/*CleanupWorker - a worker to delete transactiosn that are no longer valid */
func RedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	dbstore := badgerdbstore.GetStorageProvider()
	handler := func(ctx context.Context, key datastore.Key, value []byte) error {
		var wmEntity writemarker.WriteMarkerEntity
		err := json.Unmarshal(value, &wmEntity)
		if err != nil {
			return err
		}
		if wmEntity.Status != writemarker.Committed && wmEntity.ReedeemRetries < 10 {
			go GetProtocolImpl(wmEntity.AllocationID).RedeemMarker(&wmEntity)
		}
		return nil
	}
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dbstore.IteratePrefix(ctx, "wm:", handler)
		}
	}

}
