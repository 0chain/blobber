package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	sdkConstants "github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"go.uber.org/zap"
)

// UpdateFileCommand command for updating file
type UpdateFileCommand struct {
	exisitingFileRef *reference.Ref
	fileChanger      *allocation.UpdateFileChanger
	allocationChange *allocation.AllocationChange
}

// IsAuthorized validate request.
func (cmd *UpdateFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	uploadMetaString := req.FormValue("uploadMeta")

	if uploadMetaString == "" {
		// backward compatibility for old update request
		uploadMetaString = req.FormValue("updatedMeta")
	}

	err := json.Unmarshal([]byte(uploadMetaString), &cmd.fileChanger)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if cmd.fileChanger.ChunkSize <= 0 {
		cmd.fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}
	// Update GetReference to GetReferenceID
	cmd.exisitingFileRef, _ = reference.GetReferenceID(ctx, allocationObj.ID, cmd.fileChanger.Path)

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
func (cmd *UpdateFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	result := blobberhttp.UploadResult{}

	result.Filename = cmd.fileChanger.Filename

	origfile, _, err := req.FormFile("uploadFile")
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	cmd.reloadChange(connectionObj)

	fileInputData := &filestore.FileInputData{
		Name:    cmd.fileChanger.Filename,
		Path:    cmd.fileChanger.Path,
		OnCloud: cmd.exisitingFileRef.OnCloud,

		UploadOffset: cmd.fileChanger.UploadOffset,
		IsChunked:    cmd.fileChanger.ChunkSize > 0,
		IsFinal:      cmd.fileChanger.IsFinal,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, fileInputData, origfile, connectionObj.ConnectionID)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	result.Hash = fileOutputData.ContentHash
	//result.MerkleRoot = fileOutputData.MerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := connectionObj.Size

	// only update connection size when the chunk is uploaded by first time.
	if !fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
	}

	if len(cmd.fileChanger.ChunkHash) > 0 && cmd.fileChanger.ChunkHash != fileOutputData.ContentHash {
		return result, common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the file content")
	}

	// if len(cmd.fileChanger.MerkleRoot) > 0 && cmd.fileChanger.MerkleRoot != fileOutputData.MerkleRoot {
	// 	return result, common.NewError("content_merkle_root_mismatch", "Merkle root provided in the meta data does not match the file content")
	// }

	if allocationObj.BlobberSizeUsed+(allocationSize-cmd.exisitingFileRef.Size) > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.fileChanger.AllocationID = allocationObj.ID
	cmd.fileChanger.Size = allocationSize

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = allocationSize - cmd.exisitingFileRef.Size
	cmd.allocationChange.Operation = sdkConstants.FileOperationUpdate

	if cmd.fileChanger.IsFinal {
		connectionObj.Size = allocationSize - cmd.exisitingFileRef.Size
	} else {
		connectionObj.Size = allocationSize
	}

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UpdateFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	thumbfile, thumbHeader, _ := req.FormFile("uploadThumbnailFile")

	if thumbHeader != nil {
		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.fileChanger.Path}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, thumbInputData, thumbfile, connectionObj.ConnectionID)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}
		if len(cmd.fileChanger.ThumbnailHash) > 0 && cmd.fileChanger.ThumbnailHash != thumbOutputData.ContentHash {
			return common.NewError("content_hash_mismatch", "Content hash provided in the meta data does not match the thumbnail content")
		}
		cmd.fileChanger.ThumbnailHash = thumbOutputData.ContentHash
		cmd.fileChanger.ThumbnailSize = thumbOutputData.Size
		cmd.fileChanger.ThumbnailFilename = thumbInputData.Name
	}

	return nil
}

func (cmd *UpdateFileCommand) reloadChange(connectionObj *allocation.AllocationChangeCollector) {
	for _, c := range connectionObj.Changes {
		if c.Operation != constants.FileOperationUpdate {
			continue
		}

		dbFileChanger := &allocation.UpdateFileChanger{}

		err := dbFileChanger.Unmarshal(c.Input)
		if err != nil {
			logging.Logger.Error("reloadChange", zap.Error(err))
		}

		// reload uploaded size from db, it was chunk size from client
		cmd.fileChanger.Size = dbFileChanger.Size
		cmd.fileChanger.ThumbnailFilename = dbFileChanger.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = dbFileChanger.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = dbFileChanger.Hash
		return
	}
}

// UpdateChange add UpdateFileChanger in db
func (cmd *UpdateFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	for _, c := range connectionObj.Changes {
		if c.Operation != constants.FileOperationUpdate {
			continue
		}

		c.Size = connectionObj.Size
		c.Input, _ = cmd.fileChanger.Marshal()

		//c.ModelWithTS.UpdatedAt = time.Now()
		err := connectionObj.Save(ctx)
		if err != nil {
			return err
		}

		return c.Save(ctx)
	}

	//NOT FOUND
	connectionObj.AddChange(cmd.allocationChange, cmd.fileChanger)

	return connectionObj.Save(ctx)
}
