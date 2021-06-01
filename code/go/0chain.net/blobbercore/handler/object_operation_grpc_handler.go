package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
)

func (b *blobberGRPCService) RenameObject(ctx context.Context, r *blobbergrpc.RenameObjectRequest) (*blobbergrpc.RenameObjectResponse, error) {
	logger := ctxzap.Extract(ctx)

	allocationTx := r.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	md := GetGRPCMetaDataFromCtx(ctx)
	valid, err := verifySignatureFromRequest(allocationTx, md.ClientSignature, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	allocationID := allocationObj.ID
	clientID := md.Client

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	newName := r.NewName
	if len(newName) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid name")
	}

	pathHash := r.PathHash
	path := r.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = b.packageHandler.GetReferenceLookup(ctx, allocationObj.ID, path)
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.
			NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	connectionID := r.ConnectionId
	if len(connectionID) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid connection id passed")
	}

	connectionObj, err := b.packageHandler.GetAllocationChanges(ctx, connectionID, allocationID, clientID)
	if err != nil {
		return nil, common.NewError("meta_error", "Error reading metadata for connection")
	}

	mutex := lock.GetMutex(connectionObj.TableName(), connectionID)
	mutex.Lock()
	defer mutex.Unlock()

	objectRef, err := b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, pathHash)

	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid file path. "+err.Error())
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ConnectionID
	allocationChange.Size = 0
	allocationChange.Operation = allocation.RENAME_OPERATION
	dfc := &allocation.RenameFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, Path: objectRef.Path}
	dfc.NewName = newName
	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = b.packageHandler.SaveAllocationChanges(ctx, connectionObj)
	if err != nil {
		logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobbergrpc.RenameObjectResponse{}
	result.Filename = newName
	result.ContentHash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}
