package readmarker

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
)

func redeemReadMarker(ctx context.Context) {
	defer func() {
		iterInprogress = false
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] redeemReadMarker", zap.Any("err", r))
		}

	}()

	if !iterInprogress {
		iterInprogress = true
		rctx := datastore.GetStore().CreateTransaction(ctx)
		db := datastore.GetStore().GetTransaction(rctx)
		readMarkers := make([]*ReadMarkerEntity, 0)
		rm := &ReadMarkerEntity{RedeemRequired: true}
		db.Where(rm). // redeem_required = true
				Where("counter <> suspend"). // and not suspended
				Order("created_at ASC").Find(&readMarkers)
		if len(readMarkers) > 0 {
			swg := sizedwaitgroup.New(config.Configuration.RMRedeemNumWorkers)
			for _, rmEntity := range readMarkers {
				swg.Add()
				go func(redeemCtx context.Context, rmEntity *ReadMarkerEntity) {
					redeemCtx = datastore.GetStore().CreateTransaction(redeemCtx)
					defer redeemCtx.Done()
					err := RedeemReadMarker(redeemCtx, rmEntity)
					if err != nil {
						logging.Logger.Error("Error redeeming the read marker.", zap.Error(err))
					}
					db := datastore.GetStore().GetTransaction(redeemCtx)
					err = db.Commit().Error
					if err != nil {
						logging.Logger.Error("Error committing the readmarker redeem", zap.Error(err))
					}
					swg.Done()
				}(ctx, rmEntity)
			}
			swg.Wait()
		}
		db.Rollback()
		rctx.Done()
		iterInprogress = false
	}
}
