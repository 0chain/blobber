package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/constants"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/readmarker"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"path/filepath"
	"strconv"
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

func (b *blobberGRPCService) RenameObject(ctx context.Context, r *blobbergrpc.RenameObjectRequest) (
	*blobbergrpc.RenameObjectResponse, error) {

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
	allocationID := allocationObj.ID

	clientID := ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	if len(clientID) == 0 {
		return nil, common.NewError("invalid_operation", "Invalid client")
	}

	if len(clientID) == 0 || allocationObj.OwnerID != clientID {
		return nil, common.
			NewError("invalid_operation", "Operation needs to be performed by the owner of the allocation")
	}

	new_name := r.NewName
	if len(new_name) == 0 {
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
	allocationChange.ConnectionID = connectionObj.GetConnectionID()
	allocationChange.Size = 0
	allocationChange.Operation = allocation.RENAME_OPERATION
	dfc := &allocation.RenameFileChange{ConnectionID: connectionObj.GetConnectionID(),
		AllocationID: connectionObj.GetAllocationID(), Path: objectRef.Path}
	dfc.NewName = new_name
	connectionObj.SetSize(connectionObj.GetSize() + allocationChange.Size)
	connectionObj.AddChange(allocationChange, dfc)

	err = connectionObj.Save(ctx)
	if err != nil {
		Logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	result := &blobbergrpc.RenameObjectResponse{}
	result.Filename = new_name
	result.ContentHash = objectRef.Hash
	result.MerkleRoot = objectRef.MerkleRoot
	result.Size = objectRef.Size

	return result, nil
}

func (b *blobberGRPCService) DownloadFile(ctx context.Context, r *blobbergrpc.DownloadFileRequest) (
	*blobbergrpc.DownloadFileResponse, error) {

	ctx = setupGRPCHandlerContext(ctx, r.Context)
	var (
		allocationTx = ctx.Value(constants.ALLOCATION_CONTEXT_KEY).(string)
		clientID     = ctx.Value(constants.CLIENT_CONTEXT_KEY).(string)

		allocationObj *allocation.Allocation
	)

	if len(clientID) == 0 {
		return nil, common.NewError("download_file", "invalid client")
	}

	// runtime type check
	_ = ctx.Value(constants.CLIENT_KEY_CONTEXT_KEY).(string)

	// verify or update allocation
	allocationObj, err := b.storageHandler.verifyAllocation(ctx, allocationTx, false)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"invalid allocation id passed: %v", err)
	}
	var allocationID = allocationObj.ID

	rxPay := r.RxPay == "true"
	pathHash := r.PathHash
	path := r.Path
	if len(pathHash) == 0 {
		if len(path) == 0 {
			return nil, common.NewError("invalid_parameters", "Invalid path")
		}
		pathHash = b.packageHandler.GetReferenceLookup(ctx, allocationObj.ID, path)
	}

	var blockNumStr = r.BlockNum
	if len(blockNumStr) == 0 {
		return nil, common.NewError("download_file", "no block number")
	}

	var blockNum int64
	blockNum, err = strconv.ParseInt(blockNumStr, 10, 64)
	if err != nil || blockNum < 0 {
		return nil, common.NewError("download_file", "invalid block number")
	}

	// we can add r.NumBlocks as int64 if len() validation is not required
	var numBlocksStr = r.NumBlocks
	if len(numBlocksStr) == 0 {
		numBlocksStr = "1"
	}

	var numBlocks int64
	numBlocks, err = strconv.ParseInt(numBlocksStr, 10, 64)
	if err != nil || numBlocks < 0 {
		return nil, common.NewError("download_file",
			"invalid number of blocks")
	}

	var (
		readMarkerString = r.ReadMarker
		readMarker       = readmarker.ReadMarker{}
	)
	err = json.Unmarshal([]byte(readMarkerString), &readMarker)
	if err != nil {
		return nil, common.NewErrorf("download_file", "invalid parameters, "+
			"error parsing the readmarker for download: %v", err)
	}

	var rmObj = &readmarker.ReadMarkerEntity{}
	rmObj.LatestRM = &readMarker

	if err = rmObj.VerifyMarker(ctx, allocationObj); err != nil {
		return nil, common.NewErrorf("download_file", "invalid read marker, "+
			"failed to verify the read marker: %v", err)
	}

	var fileref *reference.Ref
	fileref, err = b.packageHandler.GetReferenceFromLookupHash(ctx, allocationID, pathHash)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"invalid file path: %v", err)
	}
	if fileref.Type != reference.FILE {
		return nil, common.NewErrorf("download_file",
			"path is not a file: %v", err)
	}

	var (
		authTokenString       = r.AuthToken
		clientIDForReadRedeem = clientID // default payer is client
		isACollaborator       = b.packageHandler.IsACollaborator(ctx, fileref.ID, clientID)
	)
	// Owner will pay for collaborator
	if isACollaborator {
		clientIDForReadRedeem = allocationObj.OwnerID
	}

	if (allocationObj.OwnerID != clientID &&
		allocationObj.PayerID != clientID &&
		!isACollaborator) || len(authTokenString) > 0 {

		var authTicketVerified bool
		authTicketVerified, err = b.storageHandler.verifyAuthTicket(ctx, r.AuthToken, allocationObj,
			fileref, clientID)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"verifying auth ticket: %v", err)
		}

		if !authTicketVerified {
			return nil, common.NewErrorf("download_file",
				"could not verify the auth ticket")
		}

		var authToken = &readmarker.AuthTicket{}
		err = json.Unmarshal([]byte(authTokenString), &authToken)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"error parsing the auth ticket for download: %v", err)
		}

		var attrs *reference.Attributes
		if attrs, err = fileref.GetAttributes(); err != nil {
			return nil, common.NewErrorf("download_file",
				"error getting file attributes: %v", err)
		}

		// if --rx_pay used 3rd_party pays
		if rxPay {
			clientIDForReadRedeem = clientID
		} else if attrs.WhoPaysForReads == common.WhoPaysOwner {
			clientIDForReadRedeem = allocationObj.OwnerID // owner pays
		}
		readMarker.AuthTicket = datatypes.JSON(authTokenString)
	}

	var (
		rme           *readmarker.ReadMarkerEntity
		latestRM      *readmarker.ReadMarker
		pendNumBlocks int64
	)
	rme, err = b.packageHandler.GetLatestReadMarkerEntity(ctx, clientID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.NewErrorf("download_file",
			"couldn't get read marker from DB: %v", err)
	}

	if rme != nil {
		latestRM = rme.LatestRM
		if pendNumBlocks, err = rme.PendNumBlocks(); err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get number of blocks pending redeeming: %v", err)
		}
	}

	if latestRM != nil &&
		latestRM.ReadCounter+(numBlocks) != readMarker.ReadCounter {

		var response = &blobbergrpc.DownloadFileResponse{
			Success:  false,
			LatestRm: ReadMarkerToReadMarkerGRPC(*latestRM),
			//Path:         fileref.Path,
			//AllocationID: fileref.AllocationID,
		}
		return response, nil
	}

	// check out read pool tokens if read_price > 0
	err = b.storageHandler.readPreRedeem(ctx, allocationObj, numBlocks, pendNumBlocks,
		clientIDForReadRedeem)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"pre-redeeming read marker: %v", err)
	}
	// reading allowed

	var (
		downloadMode = r.Content
		respData     []byte
	)
	if len(downloadMode) > 0 && downloadMode == DOWNLOAD_CONTENT_THUMB {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ThumbnailHash
		fileData.OnCloud = fileref.OnCloud
		respData, err = b.packageHandler.GetFileStore().GetFileBlock(allocationID,
			fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get thumbnail block: %v", err)
		}
	} else {
		var fileData = &filestore.FileInputData{}
		fileData.Name = fileref.Name
		fileData.Path = fileref.Path
		fileData.Hash = fileref.ContentHash
		fileData.OnCloud = fileref.OnCloud
		respData, err = b.packageHandler.GetFileStore().GetFileBlock(allocationID,
			fileData, blockNum, numBlocks)
		if err != nil {
			return nil, common.NewErrorf("download_file",
				"couldn't get file block: %v", err)
		}
	}

	readMarker.PayerID = clientIDForReadRedeem
	err = b.packageHandler.SaveLatestReadMarker(ctx, &readMarker, latestRM == nil)
	if err != nil {
		return nil, common.NewErrorf("download_file",
			"couldn't save latest read marker: %v", err)
	}

	var response = &blobbergrpc.DownloadFileResponse{}
	response.Success = true
	response.LatestRm = ReadMarkerToReadMarkerGRPC(readMarker)
	response.Data = respData
	//response.Path = fileref.Path
	//response.AllocationID = fileref.AllocationID

	b.packageHandler.FileBlockDownloaded(ctx, fileref.ID)
	//originally here is respData, need to clarify
	return response, nil
}
