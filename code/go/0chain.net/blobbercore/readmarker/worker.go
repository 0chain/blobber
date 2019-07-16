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

func RedeemReadMarker(ctx context.Context, rmEntity *ReadMarkerEntity) error {
	Logger.Info("Redeeming the read marker", zap.Any("rm", rmEntity.LatestRM))
	params := make(map[string]string)
	params["blobber"] = rmEntity.LatestRM.BlobberID
	params["client"] = rmEntity.LatestRM.ClientID
	latestRM := ReadMarker{BlobberID: rmEntity.LatestRM.BlobberID, ClientID: rmEntity.LatestRM.ClientID}
	latestRMBytes, errsc := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params, chain.GetServerChain(), nil)
	if errsc == nil {
		errsc = json.Unmarshal(latestRMBytes, &latestRM)
		if errsc != nil {
			Logger.Error("Error from unmarshal of rm bytes", zap.Error(errsc))
		} else {
			Logger.Info("Latest read marker from blockchain", zap.Any("rm", latestRM))
			if latestRM.ReadCounter > 0 && latestRM.ReadCounter >= rmEntity.LatestRM.ReadCounter {
				Logger.Info("Updating the local state to match the block chain")
				SaveLatestReadMarker(ctx, &latestRM, false)
				rmEntity.LatestRM = &latestRM
				rmEntity.UpdateStatus(ctx, "Updating the local state to match the block chain", "sync")
				return nil
			}
		}

	} else {
		Logger.Error("Error from sc rest api call", zap.Error(errsc))
	}
	var err error
	if latestRMBytes != nil {
		err = json.Unmarshal(latestRMBytes, &latestRM)
	}
	if err == nil && latestRM.ReadCounter < rmEntity.LatestRM.ReadCounter {
		err = rmEntity.RedeemReadMarker(ctx)
		if err != nil {
			Logger.Error("Error redeeming the read marker.", zap.Any("rm", rmEntity), zap.Any("error", err))
			return err
		}
		Logger.Info("Successfully redeemed read marker", zap.Any("rm", rmEntity.LatestRM))
	}
	return nil
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
				rctx := datastore.GetStore().CreateTransaction(ctx)
				db := datastore.GetStore().GetTransaction(rctx)
				readMarkers := make([]*ReadMarkerEntity, 0)
				rm := &ReadMarkerEntity{RedeemRequired: true}
				db.Where(rm).Find(&readMarkers)
				if len(readMarkers) > 0 {
					swg := sizedwaitgroup.New(config.Configuration.RMRedeemNumWorkers)
					for _, rmEntity := range readMarkers {
						swg.Add()
						go func(redeemCtx context.Context, rmEntity *ReadMarkerEntity) {
							redeemCtx = datastore.GetStore().CreateTransaction(redeemCtx)
							defer redeemCtx.Done()
							err := RedeemReadMarker(redeemCtx, rmEntity)
							if err != nil {
								Logger.Error("Error redeeming the read marker.", zap.Error(err))
							}
							db := datastore.GetStore().GetTransaction(redeemCtx)
							err = db.Commit().Error
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
