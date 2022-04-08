package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	zLogger "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"go.uber.org/zap"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

// PendNumBlocks Return number of blocks pending to be redeemed
func (rme *ReadMarkerEntity) PendNumBlocks() (pendNumBlocks int64, err error) {
	if rme.LatestRM == nil {
		return 0, common.NewErrorf("rme_pend_num_blocks", "missing latest read marker (nil)")
	}

	pendNumBlocks = rme.LatestRM.ReadCounter - rme.LatestRedeemedRC
	return
}

// RedeemReadMarker redeems the read marker.
func (rme *ReadMarkerEntity) RedeemReadMarker(ctx context.Context) (err error) {
	tx, err := transaction.NewTransactionEntity()
	if err != nil {
		return common.NewErrorf("redeem_read_marker", "creating transaction: %v", err)
	}

	sn := &ReadRedeem{
		ReadMarker: rme.LatestRM,
	}

	var snBytes []byte
	if snBytes, err = json.Marshal(sn); err != nil {
		zLogger.Logger.Error("Error encoding SC input", zap.Error(err), zap.Any("scdata", sn))
		return common.NewErrorf("redeem_read_marker", "encoding SC data: %v", err)
	}

	err = tx.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.READ_REDEEM, string(snBytes), 0)
	if err != nil {
		zLogger.Logger.Info("Failed submitting read redeem", zap.Error(err))
		return common.NewErrorf("redeem_read_marker", "sending transaction: %v", err)
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)

	var logHash = tx.Hash // keep transaction hash for error logs
	tx, err = transaction.VerifyTransaction(tx.Hash, chain.GetServerChain())
	if err != nil {
		zLogger.Logger.Error("Error verifying the read redeem transaction", zap.Error(err), zap.String("txn", logHash))
		return common.NewErrorf("redeem_read_marker", "verifying transaction: %v", err)
	}

	err = rme.UpdateStatus(ctx, tx.TransactionOutput, tx.Hash)
	if err != nil {
		return common.NewErrorf("redeem_read_marker", "updating read marker status: %v", err)
	}

	return
}
