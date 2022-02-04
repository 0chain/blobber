package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startRedeemMarkers(ctx)
}

func RedeemReadMarker(ctx context.Context, rmEntity *ReadMarkerEntity) (err error) {
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
		chain.GetServerChain())

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

func startRedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.RMRedeemFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			redeemReadMarker(ctx)
		}
	}
}
