package writemarker

import (
	"context"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/gosdk/constants"

	"go.uber.org/zap"
)

type CommitConnection struct {
	AllocationRoot     string       `json:"allocation_root"`
	PrevAllocationRoot string       `json:"prev_allocation_root"`
	WriteMarker        *WriteMarker `json:"write_marker"`
}

// VerifyMarker verify WriteMarker's hash and check allocation_root if it is unique
func (wme *WriteMarkerEntity) VerifyMarker(ctx context.Context, dbAllocation *allocation.Allocation, co *allocation.AllocationChangeCollector) error {
	if wme == nil {
		return common.NewError("invalid_write_marker", "No Write Marker was found")
	}
	if wme.WM.PreviousAllocationRoot != dbAllocation.AllocationRoot {
		return common.NewError("invalid_write_marker", "Invalid write marker. Prev Allocation root does not match the allocation root on record")
	}
	if wme.WM.BlobberID != node.Self.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
	}

	if wme.WM.AllocationID != dbAllocation.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
	}

	if wme.WM.Size != co.Size {
		return common.NewError("write_marker_validation_failed", fmt.Sprintf("Write Marker size %v does not match the connection size %v", wme.WM.Size, co.Size))
	}

	clientPublicKey := ctx.Value(constants.ContextKeyClientKey).(string)
	if clientPublicKey == "" {
		return common.NewError("write_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || clientID != wme.WM.ClientID || clientID != co.ClientID || co.ClientID != wme.WM.ClientID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not by the same client who uploaded")
	}

	hashData := wme.WM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, wme.WM.Signature, signatureHash)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		Logger.Error("write_marker_sig_error", zap.Any("wm", wme.WM))
		return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
	}

	return nil
}

func (wme *WriteMarkerEntity) RedeemMarker(ctx context.Context) error {
	if len(wme.CloseTxnID) > 0 {
		t, err := transaction.VerifyTransaction(wme.CloseTxnID, chain.GetServerChain())
		if err == nil {
			wme.Status = Committed
			wme.StatusMessage = t.TransactionOutput
			wme.CloseTxnID = t.Hash
			err = wme.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash)
			return err
		}
	}

	txn, err := transaction.NewTransactionEntity()
	if err != nil {
		wme.StatusMessage = "Error creating transaction entity. " + err.Error()
		wme.ReedeemRetries++
		if err := wme.UpdateStatus(ctx, Failed, "Error creating transaction entity. "+err.Error(), ""); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	sn := &CommitConnection{}
	sn.AllocationRoot = wme.WM.AllocationRoot
	sn.PrevAllocationRoot = wme.WM.PreviousAllocationRoot
	sn.WriteMarker = &wme.WM

	err = txn.ExecuteSmartContract(transaction.STORAGE_CONTRACT_ADDRESS, transaction.CLOSE_CONNECTION_SC_NAME, sn, 0)
	if err != nil {
		Logger.Error("Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wme.Status = Failed
		wme.StatusMessage = "Failed during sending close connection to the miner. " + err.Error()
		wme.ReedeemRetries++
		if err := wme.UpdateStatus(ctx, Failed, "Failed during sending close connection to the miner. "+err.Error(), ""); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	time.Sleep(transaction.SLEEP_FOR_TXN_CONFIRMATION * time.Second)
	t, err := transaction.VerifyTransactionWithNonce(txn.Hash, txn.GetTransaction().GetTransactionNonce())
	if err != nil {
		Logger.Error("Error verifying the close connection transaction", zap.String("err:", err.Error()), zap.String("txn", txn.Hash))
		wme.Status = Failed
		wme.StatusMessage = "Error verifying the close connection transaction." + err.Error()
		wme.ReedeemRetries++
		wme.CloseTxnID = txn.Hash
		// TODO Is this single try?
		if err := wme.UpdateStatus(ctx, Failed, "Error verifying the close connection transaction."+err.Error(), txn.Hash); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}
	wme.Status = Committed
	wme.StatusMessage = t.TransactionOutput
	wme.CloseTxnID = t.Hash
	err = wme.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash)
	return err
}

func (wme *WriteMarkerEntity) VerifyRollbackMarker(ctx context.Context, dbAllocation *allocation.Allocation) error {

	if wme == nil {
		return common.NewError("invalid_write_marker", "No Write Marker was found")
	}
	if wme.WM.PreviousAllocationRoot != wme.WM.AllocationRoot {
		return common.NewError("invalid_write_marker", "Invalid write marker. Prev Allocation root does not match the allocation root of write marker")
	}
	if wme.WM.BlobberID != node.Self.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the blobber")
	}

	if wme.WM.AllocationID != dbAllocation.ID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not for the same allocation transaction")
	}

	if wme.WM.Size != 0 {
		return common.NewError("empty write_marker_validation_failed", fmt.Sprintf("Write Marker size is %v but should be 0", wme.WM.Size))
	}

	clientPublicKey := ctx.Value(constants.ContextKeyClientKey).(string)
	if clientPublicKey == "" {
		return common.NewError("write_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || clientID != wme.WM.ClientID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not by the same client who uploaded")
	}

	hashData := wme.WM.GetHashData()
	signatureHash := encryption.Hash(hashData)
	sigOK, err := encryption.Verify(clientPublicKey, wme.WM.Signature, signatureHash)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Error during verifying signature. "+err.Error())
	}
	if !sigOK {
		return common.NewError("write_marker_validation_failed", "Write marker signature is not valid")
	}

	return nil
}
