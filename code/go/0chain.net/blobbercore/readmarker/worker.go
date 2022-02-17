package readmarker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	zLogger "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

func RedeemReadMarker(ctx context.Context, db *gorm.DB, rme *ReadMarkerEntity) error {
	// Check if error is "already redeemed" then return nil
	zLogger.Logger.Info("Redeeming read marker", zap.Any("rm", rme.ReadMarker))

	params := make(map[string]string)
	params["blobber"] = node.Self.ID
	params["client"] = rme.ReadMarker.ClientID

	tx, err := transaction.NewTransactionEntity()
	if err != nil {
		return common.NewErrorf("redeem_read_marker", "creating transaction: %v", err)
	}

	rdRedeem := &ReadRedeem{
		ReadMarker: rme.ReadMarker,
	}
	rdRedeemBytes, err := json.Marshal(rdRedeem)
	if err != nil {
		zLogger.Logger.Error("Error encoding SC input", zap.Error(err), zap.Any("scdata", rdRedeem))
		return common.NewErrorf("redeem_read_marker", "encoding SC data: %v", err)
	}

	if err := tx.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.READ_REDEEM,
		string(rdRedeemBytes), 0); err != nil {
		zLogger.Logger.Info("Failed submitting read redeem", zap.Error(err))
		return common.NewErrorf("redeem_read_marker", "sending transaction: %v", err)
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	tx, err = transaction.VerifyTransaction(tx.Hash, chain.GetServerChain())
	if err != nil {
		zLogger.Logger.Error("Error verifying the read redeem transaction", zap.Error(err), zap.String("txn", tx.Hash))
		return common.NewErrorf("redeem_read_marker", "verifying transaction: %v", err)
	}

	return nil
}

func redeemReadMarkers(ctx context.Context) {
	rms, err := GetRedeemRequiringRMEntities(ctx)
	if err != nil {
		zLogger.Logger.Error(err.Error())
		return
	}

	guideCh := make(chan struct{}, config.Configuration.RMRedeemNumWorkers)
	wg := sync.WaitGroup{}
	for _, rme := range rms {
		guideCh <- struct{}{}
		wg.Add(1)
		go func(rme *ReadMarkerEntity) {
			defer func() {
				<-guideCh
				wg.Done()
			}()

			rctx := datastore.GetStore().CreateTransaction(ctx)
			db := datastore.GetStore().GetTransaction(rctx)

			if err := RedeemReadMarker(rctx, db, rme); err != nil {
				zLogger.Logger.Error(err.Error())
				rme.UpdateStatus(rctx, true, true)
			} else {
				rme.UpdateStatus(rctx, false, false)
				allocation.AddToPending(db, rme.ReadMarker.ClientID, rme.ReadMarker.AllocationID, 0, -rme.ReadMarker.ReadSize)
			}

			db.Commit()
		}(rme)
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

func SetupWorkers(ctx context.Context) {
	go startRedeemMarkers(ctx)
}
