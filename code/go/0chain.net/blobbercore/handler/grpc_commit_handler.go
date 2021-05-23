package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/writemarker"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
)

// TODO parse request from gateway as form data instead of json
func (b *blobberGRPCService) Commit(ctx context.Context, req *blobbergrpc.CommitRequest) (*blobbergrpc.CommitResponse, error) {
	md := GetGRPCMetaDataFromCtx(ctx)

	allocationTx := req.Allocation
	clientID := md.Client
	clientKey := md.ClientKey
	clientKeyBytes, _ := hex.DecodeString(clientKey)

	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	allocationID := allocationObj.ID

	connectionID := req.ConnectionId
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	mutex := lock.GetMutex(allocationObj.TableName(), allocationID)
	mutex.Lock()
	defer mutex.Unlock()

	connectionObj, err := allocation.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewErrorf("invalid_parameters",
			"Invalid connection id. Connection id was not found: %v", err)
	}
	if len(connectionObj.Changes) == 0 {
		return nil, common.NewError("invalid_parameters",
			"Invalid connection id. Connection does not have any changes.")
	}

	var isACollaborator bool
	for _, change := range connectionObj.Changes {
		if change.Operation == allocation.UPDATE_OPERATION {
			updateFileChange := new(allocation.UpdateFileChange)
			if err := updateFileChange.Unmarshal(change.Input); err != nil {
				return nil, err
			}
			fileRef, err := reference.GetReference(ctx, allocationID, updateFileChange.Path)
			if err != nil {
				return nil, err
			}
			isACollaborator = reference.IsACollaborator(ctx, fileRef.ID, clientID)
			break
		}
	}

	if len(clientID) == 0 || len(clientKey) == 0 {
		return nil, common.NewError("invalid_params", "Please provide clientID and clientKey")
	}

	if (allocationObj.OwnerID != clientID || encryption.Hash(clientKeyBytes) != clientID) && !isACollaborator {
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	if allocationObj.BlobberSizeUsed+connectionObj.Size > allocationObj.BlobberSize {
		return nil, common.NewError("max_allocation_size",
			"Max size reached for the allocation with this blobber")
	}

	writeMarkerString := req.WriteMarker
	writeMarker := writemarker.WriteMarker{}
	err = json.Unmarshal([]byte(writeMarkerString), &writeMarker)
	if err != nil {
		return nil, common.NewErrorf("invalid_parameters",
			"Invalid parameters. Error parsing the writemarker for commit: %v",
			err)
	}

	var result blobbergrpc.CommitResponse
	var latestWM *writemarker.WriteMarkerEntity
	if len(allocationObj.AllocationRoot) == 0 {
		latestWM = nil
	} else {
		latestWM, err = writemarker.GetWriteMarkerEntity(ctx,
			allocationObj.AllocationRoot)
		if err != nil {
			return nil, common.NewErrorf("latest_write_marker_read_error",
				"Error reading the latest write marker for allocation: %v", err)
		}
	}

	writemarkerObj := &writemarker.WriteMarkerEntity{}
	writemarkerObj.WM = writeMarker

	err = writemarkerObj.VerifyMarker(ctx, allocationObj, connectionObj)
	if err != nil {
		result.AllocationRoot = allocationObj.AllocationRoot
		result.ErrorMessage = "Verification of write marker failed: " + err.Error()
		result.Success = false
		if latestWM != nil {
			result.WriteMarker = convert.WriteMarkerToWriteMarkerGRPC(&latestWM.WM)
		}
		return &result, common.NewError("write_marker_verification_failed", result.ErrorMessage)
	}

	var clientIDForWriteRedeem = writeMarker.ClientID
	if isACollaborator {
		clientIDForWriteRedeem = allocationObj.OwnerID
	}

	if err = writePreRedeem(ctx, allocationObj, &writeMarker, clientIDForWriteRedeem); err != nil {
		return nil, err
	}

	err = connectionObj.ApplyChanges(ctx, writeMarker.AllocationRoot)
	if err != nil {
		return nil, err
	}
	rootRef, err := reference.GetReference(ctx, allocationID, "/")
	if err != nil {
		return nil, err
	}
	allocationRoot := encryption.Hash(rootRef.Hash + ":" + strconv.FormatInt(int64(writeMarker.Timestamp), 10))

	if allocationRoot != writeMarker.AllocationRoot {
		result.AllocationRoot = allocationObj.AllocationRoot
		if latestWM != nil {
			result.WriteMarker = convert.WriteMarkerToWriteMarkerGRPC(&latestWM.WM)
		}
		result.Success = false
		result.ErrorMessage = "Allocation root in the write marker does not match the calculated allocation root. Expected hash: " + allocationRoot
		return &result, common.NewError("allocation_root_mismatch", result.ErrorMessage)
	}
	writemarkerObj.ConnectionID = connectionObj.ConnectionID
	writemarkerObj.ClientPublicKey = clientKey
	err = writemarkerObj.Save(ctx)
	if err != nil {
		return nil, common.NewError("write_marker_error", "Error persisting the write marker")
	}

	db := datastore.GetStore().GetTransaction(ctx)
	allocationUpdates := make(map[string]interface{})
	allocationUpdates["blobber_size_used"] = gorm.Expr("blobber_size_used + ?", connectionObj.Size)
	allocationUpdates["used_size"] = gorm.Expr("used_size + ?", connectionObj.Size)
	allocationUpdates["allocation_root"] = allocationRoot
	allocationUpdates["is_redeem_required"] = true

	err = db.Model(allocationObj).Updates(allocationUpdates).Error
	if err != nil {
		return nil, common.NewError("allocation_write_error", "Error persisting the allocation object")
	}
	err = connectionObj.CommitToFileStore(ctx)
	if err != nil {
		return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}

	// TODO maybe delete 'changes' field altogether, its not being used anywhere - its not even being sent as json back as response
	//result.Changes = connectionObj.Changes

	connectionObj.DeleteChanges(ctx) //nolint:errcheck // never returns an error anyway

	db.Model(connectionObj).Updates(allocation.AllocationChangeCollector{Status: allocation.CommittedConnection})

	result.AllocationRoot = allocationObj.AllocationRoot
	result.WriteMarker = convert.WriteMarkerToWriteMarkerGRPC(&writeMarker)
	result.Success = true
	result.ErrorMessage = ""

	return &result, nil
}
