package handler

import (
	"context"
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"path/filepath"
)

func (b *blobberGRPCService) UpdateObjectAttributes(ctx context.Context, r *blobbergrpc.UpdateObjectAttributesRequest) (
	response *blobbergrpc.UpdateObjectAttributesResponse, err error) {

	ctx = setupGRPCHandlerContext(ctx, r.Context)

	var (
		allocTx  = ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
		clientID = ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

		alloc *allocation.Allocation
	)

	if alloc, err = b.storageHandler.verifyAllocation(ctx, allocTx, false); err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"Invalid allocation ID passed: %v", err)
	}

	//valid, err := verifySignatureFromRequest(r, alloc.OwnerPublicKey)
	//if !valid || err != nil {
	//	return nil, common.NewError("invalid_signature", "Invalid signature")
	//}

	// runtime type check
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	if clientID == "" {
		return nil, common.NewError("update_object_attributes",
			"missing client ID")
	}

	var attributes = r.Attributes // new attributes as string
	if attributes == "" {
		return nil, common.NewError("update_object_attributes",
			"missing new attributes, pass at least {} for empty attributes")
	}

	var attrs = new(reference.Attributes)
	if err = json.Unmarshal([]byte(attributes), attrs); err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"decoding given attributes: %v", err)
	}

	pathHash := r.PathHash
	path := r.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = b.packageHandler.GetReferenceLookup(ctx, alloc.ID, path)
	}

	if alloc.OwnerID != clientID {
		return nil, common.NewError("update_object_attributes",
			"operation needs to be performed by the owner of the allocation")
	}

	var connID = r.ConnectionId
	if connID == "" {
		return nil, common.NewErrorf("update_object_attributes",
			"invalid connection id passed: %s", connID)
	}

	var conn allocation.IAllocationChangeCollector
	conn, err = b.packageHandler.GetAllocationChanges(ctx, connID, alloc.ID, clientID)
	if err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"reading metadata for connection: %v", err)
	}

	var mutex = lock.GetMutex(conn.TableName(), connID)

	mutex.Lock()
	defer mutex.Unlock()

	var ref *reference.Ref
	ref, err = b.packageHandler.GetReferenceFromLookupHash(ctx, alloc.ID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("update_object_attributes",
			"invalid file path: %v", err)
	}

	var change = new(allocation.AllocationChange)
	change.ConnectionID = conn.GetConnectionID()
	change.Operation = allocation.UPDATE_ATTRS_OPERATION

	var uafc = &allocation.AttributesChange{
		ConnectionID: conn.GetConnectionID(),
		AllocationID: conn.GetAllocationID(),
		Path:         ref.Path,
		Attributes:   attrs,
	}

	conn.AddChange(change, uafc)

	err = conn.Save(ctx)
	if err != nil {
		Logger.Error("update_object_attributes: "+
			"error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("update_object_attributes",
			"error writing the connection meta data")
	}

	// return new attributes as result
	return &blobbergrpc.UpdateObjectAttributesResponse{WhoPaysForReads: int64(attrs.WhoPaysForReads)}, nil
}

func (b *blobberGRPCService) CopyObject(ctx context.Context, r *blobbergrpc.CopyObjectRequest) (
	*blobbergrpc.CopyObjectResponse, error) {

	ctx = setupGRPCHandlerContext(ctx, r.Context)

	allocationTx := ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewError("invalid_parameters", "Invalid allocation id passed."+err.Error())
	}

	//valid, err := verifySignatureFromRequest(r, allocationObj.OwnerPublicKey)
	//if !valid || err != nil {
	//	return nil, common.NewError("invalid_signature", "Invalid signature")
	//}
	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	allocationID := allocationObj.ID

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID { //already checked clientId ?
		return nil, common.
			NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
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
	allocationChange.ConnectionID = connectionObj.GetConnectionID()
	allocationChange.Size = objectRef.Size
	allocationChange.Operation = allocation.COPY_OPERATION

	dfc := &allocation.CopyFileChange{ConnectionID: connectionObj.GetConnectionID(),
		AllocationID: connectionObj.GetAllocationID(), DestPath: r.Dest}
	dfc.SrcPath = objectRef.Path

	connectionObj.SetSize(connectionObj.GetSize() + allocationChange.Size)
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobbergrpc.CopyObjectResponse{}
	result.Filename = objectRef.Name
	result.ContentHash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}
