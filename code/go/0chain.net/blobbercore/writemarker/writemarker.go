package writemarker

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
)

func redeemWriteMarkers() {

	db := datastore.GetStore().GetDB()
	var allocations []*allocation.Allocation
	db.Where(&allocation.Allocation{IsRedeemRequired: true}).
		Find(&allocations)
	if len(allocations) > 0 {

		logging.Logger.Info("Redeem writemarkers for allocations",
			zap.Any("numOfAllocations", len(allocations)))

		swg := sizedwaitgroup.New(config.Configuration.WMRedeemNumWorkers)
		for _, allocationObj := range allocations {
			swg.Add()
			go func(allocationObj *allocation.Allocation) {
				redeemWriterMarkersForAllocation(allocationObj)
				swg.Done()
			}(allocationObj)
		}
		swg.Wait()
	}
}
