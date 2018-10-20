package blobber

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/writemarker"
	"go.uber.org/zap"

	"0chain.net/datastore"
	. "0chain.net/logging"

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
	numWms := 0
	//totalUsed := 0

	handler := func(ctx context.Context, key datastore.Key, value []byte) error {
		numWms++
		var wmEntity writemarker.WriteMarkerEntity
		err := json.Unmarshal(value, &wmEntity)
		if err != nil {
			return err
		}
		//Logger.Info("Write marker being processed", zap.Any("wm:", wmEntity))
		//totalUsed += wmEntity.WM.IntentTransactionID

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
			numWms = 0
			dbstore.IteratePrefix(ctx, "wm:", handler)
			Logger.Info("Number of write markers with the blobber.", zap.Int("num_wm:", numWms))
		}
	}

}
