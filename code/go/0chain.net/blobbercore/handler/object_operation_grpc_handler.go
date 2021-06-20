package handler

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobbergrpc"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/convert"
	"net/http"
)

func (b *blobberGRPCService) UpdateObjectAttributes(ctx context.Context, req *blobbergrpc.UpdateObjectAttributesRequest) (*blobbergrpc.UpdateObjectAttributesResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"attributes":    {req.Attributes},
	}

	resp, err := UpdateAttributesHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.UpdateObjectAttributesResponseCreator(resp), nil
}

func (b *blobberGRPCService) CopyObject(ctx context.Context, req *blobbergrpc.CopyObjectRequest) (*blobbergrpc.CopyObjectResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"dest":          {req.Dest},
	}

	resp, err := CopyHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.CopyObjectResponseCreator(resp), nil
}

func (b *blobberGRPCService) RenameObject(ctx context.Context, req *blobbergrpc.RenameObjectRequest) (*blobbergrpc.RenameObjectResponse, error) {
	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}
	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":          {req.Path},
		"path_hash":     {req.PathHash},
		"connection_id": {req.ConnectionId},
		"new_name":      {req.NewName},
	}

	resp, err := RenameHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.RenameObjectResponseCreator(resp), nil
}

func (b *blobberGRPCService) DownloadFile(ctx context.Context, req *blobbergrpc.DownloadFileRequest) (*blobbergrpc.DownloadFileResponse, error) {

	r, err := http.NewRequest("POST", "", nil)
	if err != nil {
		return nil, err
	}

	httpRequestWithMetaData(r, GetGRPCMetaDataFromCtx(ctx), req.Allocation)
	r.Form = map[string][]string{
		"path":        {req.Path},
		"path_hash":   {req.PathHash},
		"rx_pay":      {req.RxPay},
		"block_num":   {req.BlockNum},
		"num_blocks":  {req.NumBlocks},
		"read_marker": {req.ReadMarker},
		"auth_token":  {req.AuthToken},
		"content":     {req.AuthToken},
	}

	resp, err := DownloadHandler(ctx, r)
	if err != nil {
		return nil, err
	}

	return convert.DownloadFileResponseCreator(resp), nil
}

func (b *blobberGRPCService) WriteFile(ctx context.Context, r *blobbergrpc.UploadFileRequest) (*blobbergrpc.UploadFileResponse, error) {
	logger := ctxzap.Extract(ctx)
	if r.Method == "GET" {
		return nil, common.NewError("invalid_method",
			"Invalid method used for the upload URL. Use multi-part form POST / PUT / DELETE instead")
	}

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
		return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
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

	result := &blobbergrpc.UploadFileResponse{}
	mode := allocation.INSERT_OPERATION
	if r.Method == "PUT" {
		mode = allocation.UPDATE_OPERATION
	} else if r.Method == "DELETE" {
		mode = allocation.DELETE_OPERATION
	}

	if mode == allocation.DELETE_OPERATION {
		if allocationObj.OwnerID != clientID && allocationObj.PayerID != clientID {
			return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
		}
		result, err = b.DeleteFile(ctx, r, connectionObj)
		if err != nil {
			return nil, err
		}
	} else if mode == allocation.INSERT_OPERATION || mode == allocation.UPDATE_OPERATION {
		var formData allocation.UpdateFileChange
		uploadMetaString := r.UploadMeta
		if mode == allocation.UPDATE_OPERATION {
			uploadMetaString = r.UpdateMeta
		}
		err = json.Unmarshal([]byte(uploadMetaString), &formData)
		if err != nil {
			return nil, common.NewError("invalid_parameters",
				"Invalid parameters. Error parsing the meta data for upload."+err.Error())
		}
		exisitingFileRef, _ := b.packageHandler.GetReference(ctx, allocationID, formData.Path)
		existingFileRefSize := int64(0)
		exisitingFileOnCloud := false
		if mode == allocation.INSERT_OPERATION {
			if allocationObj.OwnerID != clientID && allocationObj.PayerID != clientID {
				return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
			}

			if exisitingFileRef != nil {
				return nil, common.NewError("duplicate_file", "File at path already exists")
			}
		} else if mode == allocation.UPDATE_OPERATION {
			if exisitingFileRef == nil {
				return nil, common.NewError("invalid_file_update", "File at path does not exist for update")
			}

			if allocationObj.OwnerID != clientID &&
				allocationObj.PayerID != clientID &&
				!reference.IsACollaborator(ctx, exisitingFileRef.ID, clientID) {
				return nil, common.NewError("invalid_operation", "Operation needs to be performed by the owner, collaborator or the payer of the allocation")
			}
		}

		if exisitingFileRef != nil {
			existingFileRefSize = exisitingFileRef.Size
			exisitingFileOnCloud = exisitingFileRef.OnCloud
		}

		//Files read from grpc bytes. Need to consider about file size and client side implementation for this
		//This is a grpc equivalent implementation for http multi-part form file. Need a proper review on this
		grpcOrgFile := bytes.NewReader(r.UploadFile)
		thumb := r.UploadThumbnailFile
		thumbnailPresent := thumb != nil

		fileInputData := &filestore.FileInputData{Name: formData.Filename, Path: formData.Path, OnCloud: exisitingFileOnCloud}
		fileOutputData, err := b.packageHandler.GetFileStore().WriteFileGRPC(allocationID, fileInputData, grpcOrgFile, connectionObj.ConnectionID)
		if err != nil {
			return nil, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
		}

		result.Filename = formData.Filename
		result.ContentHash = fileOutputData.ContentHash
		result.MerkleRoot = fileOutputData.MerkleRoot
		result.Size = fileOutputData.Size

		if len(formData.Hash) > 0 && formData.Hash != fileOutputData.ContentHash {
			return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
		}
		if len(formData.MerkleRoot) > 0 && formData.MerkleRoot != fileOutputData.MerkleRoot {
			return nil, common.NewError("content_merkle_root_mismatch", "Merkle root provided in the meta data does not match the file content")
		}
		if fileOutputData.Size > config.Configuration.MaxFileSize {
			return nil, common.NewError("file_size_limit_exceeded", "Size for the given file is larger than the max limit")
		}

		formData.Hash = fileOutputData.ContentHash
		formData.MerkleRoot = fileOutputData.MerkleRoot
		formData.AllocationID = allocationID
		formData.Size = fileOutputData.Size

		allocationSize := fileOutputData.Size
		if thumbnailPresent {
			thumbFile := bytes.NewReader(thumb)
			thumbInputData := &filestore.FileInputData{Name: formData.ThumbnailFilename, Path: formData.Path}
			thumbOutputData, err := b.packageHandler.GetFileStore().WriteFileGRPC(allocationID, thumbInputData, thumbFile, connectionObj.ConnectionID)
			if err != nil {
				return nil, common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
			}
			if len(formData.ThumbnailHash) > 0 && formData.ThumbnailHash != thumbOutputData.ContentHash {
				return nil, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
			}
			formData.ThumbnailHash = thumbOutputData.ContentHash
			formData.ThumbnailSize = thumbOutputData.Size
			formData.ThumbnailFilename = thumbInputData.Name
		}

		if allocationObj.BlobberSizeUsed+(allocationSize-existingFileRefSize) > allocationObj.BlobberSize {
			return nil, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
		}

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = allocationSize - existingFileRefSize
		allocationChange.Operation = mode

		connectionObj.Size += allocationChange.Size
		if mode == allocation.INSERT_OPERATION {
			connectionObj.AddChange(allocationChange, &formData.NewFileChange)
		} else if mode == allocation.UPDATE_OPERATION {
			connectionObj.AddChange(allocationChange, &formData)
		}
	}

	err = b.packageHandler.SaveAllocationChanges(ctx, connectionObj)
	if err != nil {
		logger.Error("Error in writing the connection meta data", zap.Error(err))
		return nil, common.NewError("connection_write_error", "Error writing the connection meta data")
	}

	return result, nil
}

func (b *blobberGRPCService) DeleteFile(ctx context.Context, r *blobbergrpc.UploadFileRequest, connectionObj *allocation.AllocationChangeCollector) (*blobbergrpc.UploadFileResponse, error) {
	path := r.Path
	if len(path) == 0 {
		return nil, common.NewError("invalid_parameters", "Invalid path")
	}

	fileRef, _ := b.packageHandler.GetReference(ctx, connectionObj.AllocationID, path)
	if fileRef != nil {
		deleteSize := fileRef.Size

		allocationChange := &allocation.AllocationChange{}
		allocationChange.ConnectionID = connectionObj.ConnectionID
		allocationChange.Size = 0 - deleteSize
		allocationChange.Operation = allocation.DELETE_OPERATION
		dfc := &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID,
			AllocationID: connectionObj.AllocationID, Name: fileRef.Name,
			Hash: fileRef.Hash, Path: fileRef.Path, Size: deleteSize}

		connectionObj.Size += allocationChange.Size
		connectionObj.AddChange(allocationChange, dfc)

		result := &blobbergrpc.UploadFileResponse{}
		result.Filename = fileRef.Name
		result.ContentHash = fileRef.Hash
		result.MerkleRoot = fileRef.MerkleRoot
		result.Size = fileRef.Size

		return result, nil
	}

	return nil, common.NewError("invalid_file", "File does not exist at path")
}
