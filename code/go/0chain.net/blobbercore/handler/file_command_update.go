package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	sdkConst "github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"go.uber.org/zap"
)

const (
	UpdateMeta = "updatedMeta"
)

// UpdateFileCommand command for updating file
type UpdateFileCommand struct {
	existingFileRef  *reference.Ref
	fileChanger      *allocation.UpdateFileChanger
	allocationChange *allocation.AllocationChange
	contentFile      multipart.File
	thumbFile        multipart.File
	thumbHeader      *multipart.FileHeader
}

func (cmd *UpdateFileCommand) GetExistingFileRef() *reference.Ref {
	return cmd.existingFileRef
}

func (cmd *UpdateFileCommand) GetPath() string {
	if cmd.fileChanger == nil {
		return ""
	}
	return cmd.fileChanger.Path
}

// IsValidated validate request.
func (cmd *UpdateFileCommand) IsValidated(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	uploadMetaString := req.FormValue(UploadMeta)

	if uploadMetaString == "" {
		// backward compatibility for old update request
		uploadMetaString = req.FormValue(UpdateMeta)
	}

	err := json.Unmarshal([]byte(uploadMetaString), &cmd.fileChanger)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if cmd.fileChanger.ConnectionID == "" {
		return common.NewError("invalid_connection", "Invalid connection id")
	}

	cmd.fileChanger.PathHash = encryption.Hash(cmd.fileChanger.Path)

	if cmd.fileChanger.ChunkSize <= 0 {
		cmd.fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}

	err = allocation.GetError(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash)
	if err != nil {
		return err
	}

	// Check if ref exists at start of update or get existing ref
	if cmd.fileChanger.UploadOffset == 0 {
		logging.Logger.Info("UpdateFile ref exists check")
		cmd.existingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, cmd.fileChanger.Path)
		if cmd.existingFileRef == nil {
			return common.NewError("invalid_file_update", "File at path does not exist for update")
		}
		logging.Logger.Info("UpdateFile ref exists check done", zap.Any("ref", cmd.existingFileRef))
		allocation.CreateConnectionChange(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash, allocationObj)
		err = allocation.SaveExistingRef(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash, cmd.existingFileRef)
		if err != nil {
			return common.NewError("invalid_file_update", "Error saving existing ref")
		}
	} else {
		cmd.existingFileRef = allocation.GetExistingRef(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash)
		if cmd.existingFileRef == nil {
			return common.NewError("invalid_file_update", "Existing file reference is nil")
		}
	}

	thumbFile, thumbHeader, _ := req.FormFile(UploadThumbnailFile)
	if thumbHeader != nil {
		if thumbHeader.Size > MaxThumbnailSize {
			return common.NewError("max_thumbnail_size",
				fmt.Sprintf("thumbnail size %d should not be greater than %d", thumbHeader.Size, MaxThumbnailSize))
		}
		cmd.thumbFile = thumbFile
		cmd.thumbHeader = thumbHeader
	}

	origfile, _, err := req.FormFile(UploadFile)
	if err != nil {
		return common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	cmd.contentFile = origfile
	if cmd.fileChanger.IsFinal {
		return allocation.SetFinalized(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash, cmd)
	}
	return allocation.SendCommand(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash, cmd)
}

// ProcessContent flush file to FileStorage
func (cmd *UpdateFileCommand) ProcessContent(allocationObj *allocation.Allocation) (allocation.UploadResult, error) {
	result := allocation.UploadResult{}

	result.Filename = cmd.fileChanger.Filename
	defer cmd.contentFile.Close()
	if cmd.fileChanger.IsFinal {
		cmd.reloadChange()
	}

	if cmd.fileChanger.Size == 0 {
		return result, common.NewError("invalid_parameters", "Invalid parameters. Size cannot be zero")
	}

	var hasher *filestore.CommitHasher
	filePathHash := cmd.fileChanger.PathHash
	connID := cmd.fileChanger.ConnectionID
	if cmd.fileChanger.UploadOffset == 0 {
		hasher = filestore.GetNewCommitHasher(cmd.fileChanger.Size)
		allocation.UpdateConnectionObjWithHasher(connID, filePathHash, hasher)
	} else {
		hasher = allocation.GetHasher(connID, filePathHash)
		if hasher == nil {
			return result, common.NewError("invalid_parameters", "Invalid parameters. Error getting hasher for upload.")
		}
	}

	fileInputData := &filestore.FileInputData{
		Name:         cmd.fileChanger.Filename,
		Path:         cmd.fileChanger.Path,
		UploadOffset: cmd.fileChanger.UploadOffset,
		IsFinal:      cmd.fileChanger.IsFinal,
		FilePathHash: filePathHash,
		Hasher:       hasher,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connID, fileInputData, cmd.contentFile)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	if cmd.fileChanger.IsFinal {
		err = hasher.Finalize()
		if err != nil {
			return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
		}
		result.IsFinal = true
	}

	result.ValidationRoot = fileOutputData.ValidationRoot
	result.FixedMerkleRoot = fileOutputData.FixedMerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := allocation.GetConnectionObjSize(connID)

	if fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
		allocation.UpdateConnectionObjSize(connID, fileOutputData.Size)
	}

	if allocationObj.BlobberSizeUsed+(allocationSize-cmd.existingFileRef.Size) > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.fileChanger.AllocationID = allocationObj.ID

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connID
	cmd.allocationChange.Size = cmd.fileChanger.Size - cmd.existingFileRef.Size
	cmd.allocationChange.Operation = sdkConst.FileOperationUpdate

	if cmd.fileChanger.IsFinal {
		allocation.UpdateConnectionObjSize(connID, -cmd.existingFileRef.Size)
	}

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UpdateFileCommand) ProcessThumbnail(allocationObj *allocation.Allocation) error {
	connectionID := cmd.fileChanger.ConnectionID
	if cmd.thumbHeader != nil {
		defer cmd.thumbFile.Close()

		thumbInputData := &filestore.FileInputData{Name: cmd.thumbHeader.Filename, Path: cmd.fileChanger.Path, IsThumbnail: true, FilePathHash: cmd.fileChanger.PathHash}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connectionID, thumbInputData, cmd.thumbFile)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}

		cmd.fileChanger.ThumbnailSize = thumbOutputData.Size
		cmd.fileChanger.ThumbnailFilename = thumbInputData.Name
		err = allocation.SaveFileChanger(connectionID, &cmd.fileChanger.BaseFileChanger)
		return err
	}
	return nil
}

func (cmd *UpdateFileCommand) reloadChange() {
	changer := allocation.GetFileChanger(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash)
	if changer != nil {
		cmd.fileChanger.ThumbnailFilename = changer.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = changer.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = changer.ThumbnailHash
	}
}

// UpdateChange add UpdateFileChanger in db
func (cmd *UpdateFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.fileChanger)
	return connectionObj.Save(ctx)
}
