package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
)

// InsertFileCommand command for inserting file
type InsertFileCommand struct {
	allocationChange *allocation.AllocationChange
	changeProcessor  *allocation.UpdateFileChanger
}

// IsAuthorized validate request.
func (cmd *InsertFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {

	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	changeProcessor := &allocation.UpdateFileChanger{}

	uploadMetaString := req.FormValue("uploadMeta")
	err := json.Unmarshal([]byte(uploadMetaString), changeProcessor)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}
	exisitingFileRef, _ := reference.GetReference(ctx, allocationObj.ID, changeProcessor.Path)

	if exisitingFileRef != nil {
		return common.NewError("duplicate_file", "File at path already exists")
	}

	if changeProcessor.ChunkSize <= 0 {
		changeProcessor.ChunkSize = fileref.CHUNK_SIZE
	}

	cmd.changeProcessor = changeProcessor

	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *InsertFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {

	result := blobberhttp.UploadResult{}

	origfile, _, err := req.FormFile("uploadFile")
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	fileInputData := &filestore.FileInputData{Name: cmd.changeProcessor.Filename, Path: cmd.changeProcessor.Path, OnCloud: false}
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

	if allocationObj.BlobberSizeUsed+allocationSize > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = allocationSize
	cmd.allocationChange.Operation = constants.FileOperationInsert

	connectionObj.Size += cmd.allocationChange.Size

	return result, nil

}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *InsertFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {

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

// UpdateChange add NewFileChange in db
func (cmd *InsertFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)
	return connectionObj.Save(ctx)
}
