package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/datastore"
	"0chain.net/core/chain"
	. "0chain.net/core/logging"
	"0chain.net/core/transaction"

	"github.com/remeh/sizedwaitgroup"
	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go RedeemMarkers(ctx)
}

func RedeemReadMarker(ctx context.Context, rmEntity *ReadMarkerEntity) (
	err error) {

	Logger.Info("Redeeming the read marker", zap.Any("rm", rmEntity.LatestRM))

	var params = make(map[string]string)
	params["blobber"] = rmEntity.LatestRM.BlobberID
	params["client"] = rmEntity.LatestRM.ClientID

	var (
		latestRM      = ReadMarker{BlobberID: rmEntity.LatestRM.BlobberID, ClientID: rmEntity.LatestRM.ClientID}
		latestRMBytes []byte
	)

	latestRMBytes, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params,
		chain.GetServerChain(), nil)

	if err != nil {
		Logger.Error("Error from sc rest api call", zap.Error(err))
		return // error

	} else if err = json.Unmarshal(latestRMBytes, &latestRM); err != nil {
		Logger.Error("Error from unmarshal of rm bytes", zap.Error(err))
		return // error

	} else if latestRM.ReadCounter > 0 && latestRM.ReadCounter >= rmEntity.LatestRM.ReadCounter {

		Logger.Info("updating the local state to match the block chain")
		if err = SaveLatestReadMarker(ctx, &latestRM, false); err != nil {
			return // error
		}
		rmEntity.LatestRM = &latestRM
		if err = rmEntity.Sync(ctx); err != nil {
			Logger.Error("redeem RM loop -- error syncing RM state",
				zap.Error(err))
			return // error
		}
		return // synced from blockchain, no redeeming needed
	}

	if latestRM.ReadCounter == rmEntity.LatestRM.ReadCounter {
		return // nothing to redeem
	}

	// so, now the latestRM.ReadCounter is less than rmEntity.LatestRM.ReadCounter

	if err = rmEntity.RedeemReadMarker(ctx); err != nil {
		Logger.Error("error redeeming the read marker.",
			zap.Any("rm", rmEntity), zap.Error(err))
		return
	}

	Logger.Info("successfully redeemed read marker",
		zap.Any("rm", rmEntity.LatestRM))
	return
}

var iterInprogress = false

func RedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.RMRedeemFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress {
				iterInprogress = true
				rctx := datastore.CreateTransaction(ctx)
				db := datastore.GetTransaction(rctx)
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
							redeemCtx = datastore.CreateTransaction(redeemCtx)
							defer redeemCtx.Done()
							err := RedeemReadMarker(redeemCtx, rmEntity)
							if err != nil {
								Logger.Error("Error redeeming the read marker.", zap.Error(err))
							}
							db := datastore.GetTransaction(redeemCtx)
							err = db.Commit().Error()
							if err != nil {
								Logger.Error("Error commiting the readmarker redeem", zap.Error(err))
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
	}

}
