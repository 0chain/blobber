package writemarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
)

func redeemWriteMarker(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] redeemWriteMarker", zap.Any("err", r))
		}
	}()

	rctx := datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(rctx)
	allocations := make([]*allocation.Allocation, 0)
	alloc := &allocation.Allocation{IsRedeemRequired: true}
	db.Where(alloc).Find(&allocations)
	if len(allocations) > 0 {
		swg := sizedwaitgroup.New(config.Configuration.WMRedeemNumWorkers)
		for _, allocationObj := range allocations {
			swg.Add()
			go func(redeemCtx context.Context, allocationObj *allocation.Allocation) {
				err := RedeemMarkersForAllocation(redeemCtx, allocationObj)
				if err != nil {
					logging.Logger.Error("Error redeeming the write marker for allocation.", zap.Any("allocation", allocationObj.ID), zap.Error(err))
				}
				swg.Done()
			}(ctx, allocationObj)
		}
		swg.Wait()
	}
	db.Rollback()
	rctx.Done()
}
