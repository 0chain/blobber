package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/config"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
	"github.com/0chain/gosdk/zboxcore/fileref"
)

// UpdateFileCommand command for updating file
type UpdateFileCommand struct {
	exisitingFileRef *reference.Ref
	changeProcessor  *allocation.UpdateFileChange
	allocationChange *allocation.AllocationChange
}

// IsAuthorized validate request.
func (cmd *UpdateFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	uploadMetaString := req.FormValue("updateMeta")
	err := json.Unmarshal([]byte(uploadMetaString), &cmd.changeProcessor)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if cmd.changeProcessor.ChunkSize <= 0 {
		cmd.changeProcessor.ChunkSize = fileref.CHUNK_SIZE
	}

	cmd.exisitingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, cmd.changeProcessor.Path)

	if cmd.exisitingFileRef == nil {
		return common.NewError("invalid_file_update", "File at path does not exist for update")
	}

	if allocationObj.OwnerID != clientID &&
		allocationObj.RepairerID != clientID &&
		!reference.IsACollaborator(ctx, cmd.exisitingFileRef.ID, clientID) {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner, collaborator or the payer of the allocation")
	}

	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *UpdateFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (UploadResult, error) {

	result := UploadResult{}

	origfile, _, err := req.FormFile("uploadFile")
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	fileInputData := &filestore.FileInputData{Name: cmd.changeProcessor.Filename, Path: cmd.changeProcessor.Path, OnCloud: cmd.exisitingFileRef.OnCloud}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, fileInputData, origfile, connectionObj.ConnectionID)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	result.Filename = cmd.changeProcessor.Filename
	result.Hash = fileOutputData.ContentHash
	result.MerkleRoot = fileOutputData.MerkleRoot
	result.Size = fileOutputData.Size

	if len(cmd.changeProcessor.Hash) > 0 && cmd.changeProcessor.Hash != fileOutputData.ContentHash {
		return result, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
	}
	if len(cmd.changeProcessor.MerkleRoot) > 0 && cmd.changeProcessor.MerkleRoot != fileOutputData.MerkleRoot {
		return result, common.NewError("content_merkle_root_mismatch", "Merkle root provided in the meta data does not match the file content")
	}
	if fileOutputData.Size > config.Configuration.MaxFileSize {
		return result, common.NewError("file_size_limit_exceeded", "Size for the given file is larger than the max limit")
	}

	cmd.changeProcessor.Hash = fileOutputData.ContentHash
	cmd.changeProcessor.MerkleRoot = fileOutputData.MerkleRoot
	cmd.changeProcessor.AllocationID = allocationObj.ID
	cmd.changeProcessor.Size = fileOutputData.Size

	allocationSize := fileOutputData.Size

	if allocationObj.BlobberSizeUsed+(allocationSize-cmd.exisitingFileRef.Size) > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = allocationSize - cmd.exisitingFileRef.Size
	cmd.allocationChange.Operation = allocation.UPDATE_OPERATION

	connectionObj.Size += cmd.allocationChange.Size

	return result, nil

}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UpdateFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {

	thumbfile, thumbHeader, _ := req.FormFile("uploadThumbnailFile")

	if thumbHeader != nil {

		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.changeProcessor.Path}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, thumbInputData, thumbfile, connectionObj.ConnectionID)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}
		if len(cmd.changeProcessor.ThumbnailHash) > 0 && cmd.changeProcessor.ThumbnailHash != thumbOutputData.ContentHash {
			return common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
		}
		cmd.changeProcessor.ThumbnailHash = thumbOutputData.ContentHash
		cmd.changeProcessor.ThumbnailSize = thumbOutputData.Size
		cmd.changeProcessor.ThumbnailFilename = thumbInputData.Name
	}

	return nil

}

// UpdateChange add UpdateFileChange in db
func (cmd *UpdateFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}
