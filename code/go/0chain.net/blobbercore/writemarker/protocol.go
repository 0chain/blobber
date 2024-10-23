package writemarker

import (
	"context"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/gosdk/constants"

	"github.com/0chain/gosdk/core/transaction"

	"go.uber.org/zap"
)

type CommitConnection struct {
	AllocationRoot     string       `json:"allocation_root"`
	PrevAllocationRoot string       `json:"prev_allocation_root"`
	WriteMarker        *WriteMarker `json:"write_marker"`
	ChainData          []byte       `json:"chain_data"`
}

const (
	ADD_BLOBBER_SC_NAME      = "add_blobber"
	UPDATE_BLOBBER_SC_NAME   = "update_blobber_settings"
	ADD_VALIDATOR_SC_NAME    = "add_validator"
	CLOSE_CONNECTION_SC_NAME = "commit_connection"
	READ_REDEEM              = "read_redeem"
	CHALLENGE_RESPONSE       = "challenge_response"
	BLOBBER_HEALTH_CHECK     = "blobber_health_check"
	FINALIZE_ALLOCATION      = "finalize_allocation"
	VALIDATOR_HEALTH_CHECK   = "validator_health_check"

	STORAGE_CONTRACT_ADDRESS = "6dba10422e368813802877a85039d3985d96760ed844092319743fb3a76712d7"
	timeGap                  = 180
)

// VerifyMarker verify WriteMarker's hash and check allocation_root if it is unique
func (wme *WriteMarkerEntity) VerifyMarker(ctx context.Context, dbAllocation *allocation.Allocation, co *allocation.AllocationChangeCollector, latestWM *WriteMarkerEntity) error {
	if wme == nil {
		return common.NewError("invalid_write_marker", "No Write Marker was found")
	}

	if len(wme.WM.AllocationRoot) > 64 {
		return common.NewError("write_marker_validation_failed", "AllocationRoot exceeds maximum length")
	}

	if len(wme.WM.PreviousAllocationRoot) > 64 {
		return common.NewError("write_marker_validation_failed", "PreviousAllocationRoot exceeds maximum length")
	}

	if len(wme.WM.FileMetaRoot) > 64 {
		return common.NewError("write_marker_validation_failed", "FileMetaRoot exceeds maximum length")
	}

	if len(wme.WM.AllocationID) > 64 {
		return common.NewError("write_marker_validation_failed", "AllocationID exceeds maximum length")
	}

	if len(wme.WM.BlobberID) > 64 {
		return common.NewError("write_marker_validation_failed", "BlobberID exceeds maximum length")
	}

	if len(wme.WM.ClientID) > 64 {
		return common.NewError("write_marker_validation_failed", "ClientID exceeds maximum length")
	}

	if len(wme.WM.Signature) > 64 {
		return common.NewError("write_marker_validation_failed", "Signature exceeds maximum length")
	}

	if wme.WM.AllocationRoot == dbAllocation.AllocationRoot && dbAllocation.StorageVersion != 1 {
		return common.NewError("write_marker_validation_failed", "Write Marker allocation root is the same as the allocation root on record")
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

	clientPublicKey := ctx.Value(constants.ContextKeyClientKey).(string)
	if clientPublicKey == "" {
		return common.NewError("write_marker_validation_failed", "Could not get the public key of the client")
	}

	clientID := ctx.Value(constants.ContextKeyClient).(string)
	if clientID == "" || clientID != wme.WM.ClientID || clientID != co.ClientID || co.ClientID != wme.WM.ClientID {
		return common.NewError("write_marker_validation_failed", "Write Marker is not by the same client who uploaded")
	}
	if wme.WM.Timestamp < dbAllocation.StartTime {
		return common.NewError("write_marker_validation_failed", "Write Marker timestamp is before the allocation start time")
	}

	currTime := common.Now()
	// blobber clock is allowed to be 180 seconds behind the current time
	if wme.WM.Timestamp > currTime+timeGap {
		if latestWM == nil || wme.WM.Timestamp > latestWM.WM.Timestamp+timeGap {
			return common.NewError("write_marker_validation_failed", "Write Marker timestamp is in the future")
		}
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

func (wme *WriteMarkerEntity) redeemMarker(ctx context.Context, startSeq int64) error {
	if len(wme.CloseTxnID) > 0 {
		t, err := transaction.VerifyTransaction(wme.CloseTxnID)
		if err == nil {
			wme.Status = Committed
			wme.StatusMessage = t.TransactionOutput
			wme.CloseTxnID = t.Hash
			err = wme.UpdateStatus(ctx, Committed, t.TransactionOutput, t.Hash, startSeq, wme.Sequence)
			return err
		}
	}

	var out, hash string
	var nonce int64
	txn := &transaction.Transaction{}
	sn := &CommitConnection{}
	sn.AllocationRoot = wme.WM.AllocationRoot
	sn.PrevAllocationRoot = wme.WM.PreviousAllocationRoot
	sn.WriteMarker = &wme.WM
	var err error
	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		sn.ChainData, err = GetMarkersForChain(ctx, wme.WM.AllocationID, startSeq, wme.Sequence-1)
		return err
	})
	if err != nil {
		wme.StatusMessage = "Error getting chain data. " + err.Error()
		if err := wme.UpdateStatus(ctx, Failed, "Error getting chain data. "+err.Error(), "", startSeq, wme.Sequence); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	if sn.AllocationRoot == sn.PrevAllocationRoot && wme.WM.Version != MARKER_VERSION {
		// get nonce of prev WM
		_, err = GetPreviousWM(ctx, sn.AllocationRoot, wme.WM.Timestamp)
		if err != nil {
			wme.StatusMessage = "Error getting previous write marker. " + err.Error()
			if err := wme.UpdateStatus(ctx, Failed, "Error getting previous write marker. "+err.Error(), "", startSeq, wme.Sequence); err != nil {
				Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
			}
			return err
		}

	}

	hash, out, nonce, txn, err = transaction.SmartContractTxn(STORAGE_CONTRACT_ADDRESS, transaction.SmartContractTxnData{
		Name:      CLOSE_CONNECTION_SC_NAME,
		InputArgs: sn,
	})
	if err != nil {
		Logger.Error("Failed during sending close connection to the miner. ", zap.String("err:", err.Error()))
		wme.Status = Failed
		wme.StatusMessage = "Failed during sending close connection to the miner. " + err.Error()
		if err := wme.UpdateStatus(ctx, Failed, "Failed during sending close connection to the miner. "+err.Error(), "", startSeq, wme.Sequence); err != nil {
			Logger.Error("WriteMarkerEntity_UpdateStatus", zap.Error(err))
		}
		return err
	}

	wme.CloseTxnID = txn.Hash
	wme.CloseTxnNonce = nonce
	wme.Status = Committed
	wme.StatusMessage = out
	err = wme.UpdateStatus(ctx, Committed, out, hash, startSeq, wme.Sequence)
	return err
}

func (wme *WriteMarkerEntity) VerifyRollbackMarker(ctx context.Context, dbAllocation *allocation.Allocation, latestWM *WriteMarkerEntity) error {

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

	if wme.WM.Size != -latestWM.WM.Size {
		return common.NewError("empty write_marker_validation_failed", fmt.Sprintf("Write Marker size is %v but should be 0", wme.WM.Size))
	}

	if wme.WM.ChainSize != latestWM.WM.ChainSize+wme.WM.Size {
		return common.NewError("empty write_marker_validation_failed", fmt.Sprintf("Write Marker chain size is %v but should be %v", wme.WM.ChainSize, latestWM.WM.ChainSize+wme.WM.Size))
	}

	if latestWM.Status != Committed {
		wme.WM.ChainLength = latestWM.WM.ChainLength
	}

	if wme.WM.AllocationRoot == dbAllocation.AllocationRoot {
		return common.NewError("write_marker_validation_failed", "Write Marker allocation root is the same as the allocation root on record")
	}

	if wme.WM.AllocationRoot != latestWM.WM.PreviousAllocationRoot {
		return common.NewError("write_marker_validation_failed", fmt.Sprintf("Write Marker allocation root %v does not match the previous allocation root of latest write marker %v", wme.WM.AllocationRoot, latestWM.WM.PreviousAllocationRoot))
	}

	prevWM, err := GetWriteMarkerEntity(ctx, dbAllocation.ID, latestWM.WM.PreviousAllocationRoot)
	if err != nil {
		return common.NewError("write_marker_validation_failed", "Error getting previous write marker. "+err.Error())
	}
	if wme.WM.FileMetaRoot != prevWM.WM.FileMetaRoot {
		return common.NewError("write_marker_validation_failed", fmt.Sprintf("Write Marker file meta root %v does not match the file meta root of previous write marker %v", wme.WM.FileMetaRoot, prevWM.WM.FileMetaRoot))
	}

	if wme.WM.Timestamp != latestWM.WM.Timestamp {
		return common.NewError("write_marker_validation_failed", fmt.Sprintf("Write Marker timestamp %v does not match the timestamp of latest write marker %v", wme.WM.Timestamp, latestWM.WM.Timestamp))
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
	wme.WM.ChainLength += 1
	return nil
}
