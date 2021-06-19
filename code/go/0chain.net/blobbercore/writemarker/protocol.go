package writemarker

import (
	"context"
	"encoding/json"
	"fmt"
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
		return common.NewError("write_marker_validation_failed", fmt.Sprintf("Write Marker size does not match the connection size %v <> %v", wm.WM.Size, co.Size))
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

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		wm.StatusMessage = "Error creating transaction entity. " + err.Error()
		wm.ReedeemRetries++
		if err := wm.UpdateStatus(ctx, Failed, "Error creating transaction entity. "+err.Error(), ""); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	sn := &CommitConnection{}
	sn.AllocationRoot = wm.WM.AllocationRoot
	sn.PrevAllocationRoot = wm.WM.PreviousAllocationRoot
	sn.WriteMarker = &wm.WM

	snBytes, err := json.Marshal(sn)
	if err != nil {
		Logger.Error("Error encoding sc input", zap.String("err:", err.Error()), zap.Any("scdata", sn))
		wm.Status = Failed
		wm.StatusMessage = "Error encoding sc input. " + err.Error()
		wm.ReedeemRetries++
		if err := wm.UpdateStatus(ctx, Failed, "Error encoding sc input. "+err.Error(), ""); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.CLOSE_CONNECTION_SC_NAME, string(snBytes), 0)
	if err != nil {
		Logger.Error("Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wm.Status = Failed
		wm.StatusMessage = "Failed during sending close connection to the miner. " + err.Error()
		wm.ReedeemRetries++
		if err := wm.UpdateStatus(ctx, Failed, "Failed during sending close connection to the miner. "+err.Error(), ""); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransaction(txn.Hash, chain.GetServerChain())
	if err != nil {
		Logger.Error("Error verifying the close connection transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		wm.Status = Failed
		wm.StatusMessage = "Error verifying the close connection transaction." + err.Error()
		wm.ReedeemRetries++
		wm.CloseTxnID = txn.Hash
		if err := wm.UpdateStatus(ctx, Failed, "Error verifying the close connection transaction."+err.Error(), txn.Hash); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}
	wm.Status = Committed
	wm.StatusMessage = t.TransactionOutput
	wm.CloseTxnID = t.Hash
	err = wm.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash)
	return err
}
