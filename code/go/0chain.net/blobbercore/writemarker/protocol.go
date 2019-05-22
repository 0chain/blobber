package writemarker

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

type CommitConnection struct {
	AllocationRoot     string       `json:"allocation_root"`
	PrevAllocationRoot string       `json:"prev_allocation_root"`
	WriteMarker        *WriteMarker `json:"write_marker"`
}

func (wm *WriteMarkerEntity) VerifyMarker(ctx context.Context, sa *allocation.Allocation, co *allocation.AllocationChangeCollector) error {
	if wm == nil {
		return common.NewError("invalid_write_marker", "No Write Marker was found")
	}
	if wm.WM.PreviousAllocationRoot != sa.AllocationRoot {
		return common.NewError("invalid_write_marker", "Invalid write marker. Prev Allocation root does not match the allocation root on record")
	}
	if wm.WM.BlobberID != node.Self.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
	}

	wmEntity, err := GetWriteMarkerEntity(ctx, wm.WM.AllocationRoot)

	if err == nil && wmEntity.Status != Failed {
		return common.NewError("write_marker_validation_failed", "Duplicate write marker. Validation failed")
	}

	if wm.WM.AllocationID != sa.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
	}

	if wm.WM.Size != co.Size {
		return common.NewError("write_marker_validation_failed", "Write Marker size does not match the connection size")
	}

	clientPublicKey := ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)
	if len(clientPublicKey) == 0 {
		return common.NewError("write_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	if len(clientID) == 0 || clientID != wm.WM.ClientID || clientID != co.ClientID || co.ClientID != wm.WM.ClientID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not by the same client who uploaded")
	}

	hashData := wm.WM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	Logger.Info("Computed the hash for verifying wm signature. ", zap.String("hashdata", hashData), zap.String("hash", signatureHash))
	sigOK, err := encryption.Verify(clientPublicKey, wm.WM.Signature, signatureHash)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
	}

	return nil
}

func (wm *WriteMarkerEntity) RedeemMarker(ctx context.Context) error {

	if len(wm.CloseTxnID) > 0 {
		t, err := transaction.VerifyTransaction(wm.CloseTxnID, chain.GetServerChain())
		if err == nil {
			wm.Status = Committed
			wm.StatusMessage = t.TransactionOutput
			wm.CloseTxnID = t.Hash
			err = wm.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash)
			return err
		}
	}

	txn := transaction.NewTransactionEntity()
	sn := &CommitConnection{}
	sn.AllocationRoot = wm.WM.AllocationRoot
	sn.PrevAllocationRoot = wm.WM.PreviousAllocationRoot
	sn.WriteMarker = &wm.WM

	scData := &transaction.SmartContractTxnData{}
	scData.Name = transaction.CLOSE_CONNECTION_SC_NAME
	scData.InputArgs = sn

	txn.ToClientID = transaction.STORAGE_CONTRACT_ADDRESS
	txn.Value = 0
	txn.TransactionType = transaction.TxnTypeSmartContract
	txnBytes, err := json.Marshal(scData)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", scData))
		wm.Status = Failed
		wm.StatusMessage = "Error encoding sc input. " + err.Error()
		wm.ReedeemRetries++
		err = wm.UpdateStatus(ctx, Failed, "Error encoding sc input. "+err.Error(), "")
		return err
	}
	txn.TransactionData = string(txnBytes)

	err = txn.ComputeHashAndSign()
	if err != nil {
		Logger.Error("Signing Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wm.Status = Failed
		wm.StatusMessage = "Signing Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		err = wm.UpdateStatus(ctx, Failed, "Signing Failed during sending close connection to the miner. "+err.Error(), "")
		return err
	}
	transaction.SendTransactionSync(txn, chain.GetServerChain())
	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the close connection transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		wm.Status = Failed
		wm.StatusMessage = "Error verifying the close connection transaction." + err.Error()
		wm.ReedeemRetries++
		wm.CloseTxnID = txn.Hash
		err = wm.UpdateStatus(ctx, Failed, "Error verifying the close connection transaction."+err.Error(), txn.Hash)
		return err
	}
	wm.Status = Committed
	wm.StatusMessage = t.TransactionOutput
	wm.CloseTxnID = t.Hash
	err = wm.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash)
	return err
}
