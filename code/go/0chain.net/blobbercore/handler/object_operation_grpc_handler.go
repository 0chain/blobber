package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap/ctxzap"
	"go.uber.org/zap"
	"path/filepath"
)

func (b *blobberGRPCService) CopyObject(ctx context.Context, r *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {
	logger := ctxzap.Extract(ctx)
	md := GetGRPCMetaDataFromCtx(ctx)

	allocationTx := r.Allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	valid, err := verifySignatureFromRequest(allocationTx, md.ClientSignature, allocationObj.OwnerPublicKey)
	if !valid || err != nil {
		return nil, common.NewError("invalid_signature", "Invalid signature")
	}

	allocationID := allocationObj.ID

	clientID := md.Client
	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	if len(r.Dest) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid destination for operation")
	}

	pathHash := r.PathHash
	path := r.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = b.packageHandler.GetReferenceLookup(ctx, allocationObj.ID, path)
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID { //already checked clientId ?
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

	newPath := filepath.Join(r.Dest, objectRef.Name)
	destRef, _ := b.packageHandler.GetReference(ctx, allocationID, newPath)
	if destRef != nil {
		return nil, common.NewError(
			"invalid_parameters", "Invalid destination path. Object Already exists.")
	}

	destRef, err = b.packageHandler.GetReference(ctx, allocationID, r.Dest)
	if err != nil || destRef.Type != reference.DIRECTORY {
		return nil, common.NewError(
			"invalid_parameters", "Invalid destination path. Should be a valid directory.")
	}

	allocationChange := &allocation.AllocationChange{}
	allocationChange.ConnectionID = connectionObj.ConnectionID
	allocationChange.Size = objectRef.Size
	allocationChange.Operation = allocation.COPY_OPERATION

	dfc := &allocation.CopyFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, DestPath: r.Dest}
	dfc.SrcPath = objectRef.Path
	connectionObj.Size += allocationChange.Size
	connectionObj.AddChange(allocationChange, dfc)

	err = b.packageHandler.SaveAllocationChanges(ctx, connectionObj)
	if err != nil {
		logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobbergrpc.CopyObjectResponse{}
	result.Filename = objectRef.Name
	result.ContentHash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}
