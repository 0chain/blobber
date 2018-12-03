package blobber

import (
	"context"
	"fmt"
	"time"

	"0chain.net/badgerdbstore"
	"0chain.net/datastore"
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
		fmt.Println(string(key))
		fmt.Println(string(value))
		return nil
	}
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dbstore.Iterate(ctx, handler)
		}
	}

}
