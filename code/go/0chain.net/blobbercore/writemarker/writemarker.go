package writemarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"golang.org/x/sync/semaphore"
)

// func redeemWriteMarkers() {

// 	db := datastore.GetStore().GetDB()
// 	var allocations []*allocation.Allocation
// 	db.Where(&allocation.Allocation{IsRedeemRequired: true}).
// 		Find(&allocations)
// 	if len(allocations) > 0 {

// 		logging.Logger.Info("Redeem writemarkers for allocations",
// 			zap.Any("numOfAllocations", len(allocations)))

// 		swg := sizedwaitgroup.New(config.Configuration.WMRedeemNumWorkers)
// 		for _, allocationObj := range allocations {
// 			swg.Add()
// 			go func(allocationObj *allocation.Allocation) {
// 				redeemWriterMarkersForAllocation(allocationObj)
// 				swg.Done()
// 			}(allocationObj)
// 		}
// 		swg.Wait()
// 	}
// }

func startRedeemWorker(ctx context.Context) {

	sem := semaphore.NewWeighted(int64(config.Configuration.WMRedeemNumWorkers))
	for {
		select {
		case <-ctx.Done():
			return
		case wm := <-writeMarkerChan:
			err := sem.Acquire(ctx, 1)
			if err == nil {
				go func() {
					redeemWriteMarker(wm)
					sem.Release(1)
				}()
			}
		}
	}
}
