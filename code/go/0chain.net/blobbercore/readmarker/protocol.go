package readmarker

import (
	"context"
	"encoding/json"
	"time"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/constants"

	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/encryption"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

	"go.uber.org/zap"
)

type ReadRedeem struct {
	ReadMarker *ReadMarker `json:"read_marker"`
}

func (rm *ReadMarkerEntity) VerifyMarker(ctx context.Context, sa *allocation.Allocation) error {
	if rm == nil || rm.LatestRM == nil {
		return common.NewError("invalid_read_marker", "No read marker was found")
	}
	if rm.LatestRM.AllocationID != sa.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the same allocation")
	}

	if rm.LatestRM.BlobberID != node.Self.ID {
		return common.NewError("read_marker_validation_failed", "Read Marker is not for the blobber")
	}

	clientPublicKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	if len(clientPublicKey) == 0 || clientPublicKey != rm.LatestRM.ClientPublicKey {
		return common.NewError("read_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || clientID != rm.LatestRM.ClientID {
		return common.NewError("read_marker_validation_failed", "Read Marker clientID does not match request clientID")
	}
	currentTS := common.Now()
	if rm.LatestRM.Timestamp > currentTS {
		Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
	}
	currentTS = common.Now()
	if rm.LatestRM.Timestamp > (currentTS + 2) {
		Logger.Error("Timestamp is for future in the read marker", zap.Any("rm", rm), zap.Any("now", currentTS))
		return common.NewError("read_marker_validation_failed", "Timestamp is for future in the read marker")
	}

	hashData := rm.LatestRM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, rm.LatestRM.Signature, signatureHash)
	if err != nil {
		return common.NewError("read_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("read_marker_validation_failed", "Read marker signature is not valid")
	}
	return nil
}

func (rmEntity *ReadMarkerEntity) RedeemReadMarker(ctx context.Context) error {
	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		return err
	}
	rm := rmEntity.LatestRM

	sn := &ReadRedeem{}
	sn.ReadMarker = rm

	snBytes, err := json.Marshal(sn)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", sn))
		return err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.READ_REDEEM, string(snBytes), 0)
	if err != nil {
		Logger.Info("Failed submitting read redeem", zap.String("err:", err.Error()))
		return err
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the read redeem transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		return err
	}
	err = rmEntity.UpdateStatus(ctx, t.TransactionOutput, t.Hash)
	return err
}
