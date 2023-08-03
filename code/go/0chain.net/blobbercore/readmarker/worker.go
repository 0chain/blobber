package readmarker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"go.uber.org/zap"
)

func SetupWorkers(ctx context.Context) {
	go startRedeemMarkers(ctx)
}

func redeemReadMarker(ctx context.Context, rmEntity *ReadMarkerEntity) (err error) {
	logging.Logger.Info("Redeeming the read marker", zap.Any("rm", rmEntity.LatestRM))

	params := map[string]string{
		"blobber":    rmEntity.LatestRM.BlobberID,
		"client":     rmEntity.LatestRM.ClientID,
		"allocation": rmEntity.LatestRM.AllocationID,
	}

	latestRM := ReadMarker{BlobberID: rmEntity.LatestRM.BlobberID, ClientID: rmEntity.LatestRM.ClientID}
	latestRMBytes, err := transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS, "/latestreadmarker", params,
		chain.GetServerChain())

	if err != nil {
		logging.Logger.Error("Error from sc rest api call", zap.Error(err))
		return
	} else if err = json.Unmarshal(latestRMBytes, &latestRM); err != nil {
		logging.Logger.Error("Error from unmarshal of rm bytes", zap.Error(err))
		return
	} else if latestRM.ReadCounter > 0 && latestRM.ReadCounter >= rmEntity.LatestRM.ReadCounter {
		logging.Logger.Info("updating the local state to match the block chain")
		key := rmEntity.LatestRM.ClientID + ":" + rmEntity.LatestRM.AllocationID
		lock, isNewLock := ReadmarkerMapLock.GetLock(key)
		if !isNewLock {
			return fmt.Errorf("lock exists for key: %v", key)
		}

		lock.Lock()
		defer lock.Unlock()

		if err = SaveLatestReadMarker(ctx, &latestRM, latestRM.ReadCounter, false); err != nil {
			return
		}

		rmEntity.LatestRM = &latestRM
		if err = rmEntity.Sync(ctx); err != nil {
			logging.Logger.Error("redeem RM loop -- error syncing RM state", zap.Error(err))
			return
		}
		return // synced from blockchain, no redeeming needed
	}

	// so, now the latestRM.ReadCounter is less than rmEntity.LatestRM.ReadCounter

	if err = rmEntity.RedeemReadMarker(ctx); err != nil {
		logging.Logger.Error("error redeeming the read marker.", zap.Any("rm", rmEntity), zap.Error(err))
		return
	}

	logging.Logger.Info("successfully redeemed read marker", zap.Any("rm", rmEntity.LatestRM))
	return
}

func redeemReadMarkers(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] redeemReadMarker", zap.Any("err", r))
		}
	}()

	var readMarkers []*ReadMarkerEntity
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		var err error
		readMarkers, err = GetRedeemRequiringRMEntities(ctx)
		return err
	})

	if err != nil {
		logging.Logger.Error("redeem_readmarker", zap.Any("database_error", err))
		return
	}

	guideCh := make(chan struct{}, config.Configuration.RMRedeemNumWorkers)
	wg := sync.WaitGroup{}

	for _, rmEntity := range readMarkers {
		guideCh <- struct{}{}
		wg.Add(1)

		rmEntity.LatestRM.BlobberID = node.Self.ID
		go func(redeemCtx context.Context, rmEntity *ReadMarkerEntity, wg *sync.WaitGroup, ch <-chan struct{}) {
			defer func() {
				<-ch
				wg.Done()
			}()

			err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
				return redeemReadMarker(ctx, rmEntity)
			})
			if err != nil {
				logging.Logger.Error("Error redeeming the read marker.", zap.Error(err))
				return
			}
		}(ctx, rmEntity, &wg, guideCh)
	}
	wg.Wait()
}

func startRedeemMarkers(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(config.Configuration.RMRedeemFreq) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			redeemReadMarkers(ctx)
		}
	}
}
