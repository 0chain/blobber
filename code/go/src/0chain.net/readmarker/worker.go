package readmarker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"0chain.net/chain"
	"0chain.net/common"
	"0chain.net/config"
	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/lock"
	. "0chain.net/logging"
	"0chain.net/transaction"

	"go.uber.org/zap"
)

var dbstore datastore.Store
var fileStore filestore.FileStore

func SetupWorkers(ctx context.Context, metaStore datastore.Store, fsStore filestore.FileStore) {
	dbstore = metaStore
	fileStore = fsStore
	go RedeemMarkers(ctx)
}

func RedeemReadMarker(ctx context.Context, rmEntity *ReadMarkerEntity) error {
	rmStatus := &ReadMarkerStatus{}
	rmStatus.LastestRedeemedRM = &ReadMarker{ClientID: rmEntity.LatestRM.ClientID, BlobberID: rmEntity.LatestRM.BlobberID}
	mutex := lock.GetMutex(rmStatus.GetKey())
	mutex.Lock()
	defer mutex.Unlock()
	err := rmStatus.Read(ctx, rmStatus.GetKey())

	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}

	if (err != nil && err == datastore.ErrKeyNotFound) || (err == nil && rmStatus.LastestRedeemedRM.ReadCounter < rmEntity.LatestRM.ReadCounter) {
		Logger.Info("Redeeming the read marker", zap.Any("rm", rmEntity.LatestRM))
		params := make(map[string]string)
		params["blobber"] = rmEntity.LatestRM.BlobberID
		params["client"] = rmEntity.LatestRM.ClientID
		var latestRM ReadMarker
		latestRMBytes, errsc := transaction.MakeSCRestAPICall(transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params, chain.GetServerChain(), nil)
		if errsc == nil {
			errsc = json.Unmarshal(latestRMBytes, &latestRM)
			if errsc != nil {
				Logger.Error("Error from unmarshal of rm bytes", zap.Error(errsc))
			} else {
				Logger.Info("Latest read marker from blockchain", zap.Any("rm", latestRM))
				if latestRM.ReadCounter > 0 && latestRM.ReadCounter >= rmEntity.LatestRM.ReadCounter {
					Logger.Info("Updating the local state to match the block chain")
					rmStatus.LastestRedeemedRM = rmEntity.LatestRM
					rmStatus.LastRedeemTxnID = "sync"
					rmStatus.Write(ctx)
					return nil
				}
			}

		} else {
			Logger.Error("Error from sc rest api call", zap.Error(errsc))
		}
		err := rmEntity.RedeemReadMarker(ctx, rmStatus)
		if err != nil {
			Logger.Error("Error redeeming the read marker.", zap.Any("rm", rmEntity), zap.Any("error", err))
			return err
		}
		Logger.Info("Successfully redeemed read marker", zap.Any("rm", rmEntity.LatestRM), zap.Any("rm_status", rmStatus))
	}
	return nil
}

var rmHandler = func(ctx context.Context, key datastore.Key, value []byte) error {
	rmEntity := Provider().(*ReadMarkerEntity)
	err := json.Unmarshal(value, rmEntity)
	if err != nil {
		Logger.Error("Error unmarshal of the read marker entity")
		return nil
	}
	if len(rmToProcess) > 0 && rmToProcess != rmEntity.GetKey() {
		return nil
	}
	rmStatus := &ReadMarkerStatus{}
	rmStatus.LastestRedeemedRM = &ReadMarker{ClientID: rmEntity.LatestRM.ClientID, BlobberID: rmEntity.LatestRM.BlobberID}
	err = rmStatus.Read(ctx, rmStatus.GetKey())

	if err != nil && err != datastore.ErrKeyNotFound {
		Logger.Error("Error reading the read marker status." + rmStatus.GetKey())
		return nil
	}
	if rmEntity.LatestRM != nil && rmStatus.LastestRedeemedRM.ReadCounter < rmEntity.LatestRM.ReadCounter {
		if numOfWorkers < config.Configuration.RMRedeemNumWorkers {
			numOfWorkers++
			redeemWorker.Add(1)
			go func(redeemCtx context.Context) {
				redeemCtx = dbstore.WithConnection(redeemCtx)
				defer redeemCtx.Done()
				err := RedeemReadMarker(redeemCtx, rmEntity)
				if err != nil {
					Logger.Error("Error redeeming the read marker.", zap.Error(err))
				}
				err = dbstore.Commit(redeemCtx)
				if err != nil {
					Logger.Error("Error commiting the readmarker redeem", zap.Error(err))
				}
				redeemWorker.Done()
			}(context.Background())
		} else {
			rmToProcess = rmEntity.GetKey()
			return common.NewError("iter_break", "breaking the iteration")
		}

	}
	return nil
}

var redeemWorker sync.WaitGroup
var numOfWorkers = 0
var iterInprogress = false
var rmToProcess = ""

func RedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.RMRedeemFreq) * time.Second)
	for true {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !iterInprogress && numOfWorkers == 0 {
				iterInprogress = true
				rctx := dbstore.WithReadOnlyConnection(ctx)
				err := dbstore.IteratePrefix(rctx, "rm:", rmHandler)
				if err == nil {
					rmToProcess = ""
				}
				redeemWorker.Wait()
				iterInprogress = false
				numOfWorkers = 0
				dbstore.Discard(rctx)
				rctx.Done()
			}
		}
	}

}
