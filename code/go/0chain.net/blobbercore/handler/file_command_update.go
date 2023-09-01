package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
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
	logging.Logger.Info("UpdateFileCommand", zap.Any("allocation_id", allocationObj.ID), zap.Any("validation_rooot", cmd.fileChanger.ValidationRoot), zap.Any("thumb_hash", cmd.fileChanger.ThumbnailHash))

	if cmd.fileChanger.ChunkSize <= 0 {
		cmd.fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}
	cmd.existingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, cmd.fileChanger.Path)

	if cmd.existingFileRef == nil {
		return common.NewError("invalid_file_update", "File at path does not exist for update")
	}

	if allocationObj.OwnerID != clientID &&
		allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	_, thumbHeader, _ := req.FormFile(UploadThumbnailFile)
	if thumbHeader != nil {
		if thumbHeader.Size > MaxThumbnailSize {
			return common.NewError("max_thumbnail_size",
				fmt.Sprintf("thumbnail size %d should not be greater than %d", thumbHeader.Size, MaxThumbnailSize))
		}
	}

	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *UpdateFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	result := blobberhttp.UploadResult{}

	result.Filename = cmd.fileChanger.Filename

	isFinal := cmd.fileChanger.IsFinal
	cmd.fileChanger.IsFinal = false
	cmd.reloadChange(connectionObj)
	if cmd.fileChanger.IsFinal {
		return result, nil
	}
	cmd.fileChanger.IsFinal = isFinal

	origfile, _, err := req.FormFile(UploadFile)
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	if cmd.fileChanger.Size == 0 {
		return result, common.NewError("invalid_parameters", "Invalid parameters. Size cannot be zero")
	}

	var hasher *filestore.CommitHasher
	filePathHash := encryption.Hash(cmd.fileChanger.Path)
	if cmd.fileChanger.UploadOffset == 0 {
		hasher = filestore.GetNewCommitHasher(cmd.fileChanger.Size)
		allocation.UpdateConnectionObjWithHasher(connectionObj.ID, filePathHash, hasher)
	} else {
		hasher = allocation.GetHasher(connectionObj.ID, filePathHash)
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
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connectionObj.ID, fileInputData, origfile)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	if cmd.fileChanger.IsFinal {
		err = hasher.Finalize()
		if err != nil {
			return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
		}
	}

	result.ValidationRoot = fileOutputData.ValidationRoot
	result.FixedMerkleRoot = fileOutputData.FixedMerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := connectionObj.Size

	if fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
		allocation.UpdateConnectionObjSize(connectionObj.ID, fileOutputData.Size)
	}

	if allocationObj.BlobberSizeUsed+(allocationSize-cmd.existingFileRef.Size) > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.fileChanger.AllocationID = allocationObj.ID
	// cmd.fileChanger.Size += fileOutputData.Size

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ID
	cmd.allocationChange.Size = cmd.fileChanger.Size - cmd.existingFileRef.Size
	cmd.allocationChange.Operation = sdkConst.FileOperationUpdate

	if cmd.fileChanger.IsFinal {
		connectionObj.Size = allocationSize - cmd.existingFileRef.Size
		allocation.UpdateConnectionObjSize(connectionObj.ID, -cmd.existingFileRef.Size)
	} else {
		connectionObj.Size = allocationSize
	}

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UpdateFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	thumbfile, thumbHeader, _ := req.FormFile(UploadThumbnailFile)

	if thumbHeader != nil {
		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.fileChanger.Path, IsThumbnail: true, FilePathHash: encryption.Hash(cmd.fileChanger.Path)}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connectionObj.ID, thumbInputData, thumbfile)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}

		cmd.fileChanger.ThumbnailSize = thumbOutputData.Size
		cmd.fileChanger.ThumbnailFilename = thumbInputData.Name
	}

	return nil
}

func (cmd *UpdateFileCommand) reloadChange(connectionObj *allocation.AllocationChangeCollector) {
	for _, c := range connectionObj.Changes {
		filePath, _ := c.GetOrParseAffectedFilePath()
		if c.Operation != sdkConst.FileOperationUpdate || cmd.fileChanger.Path != filePath {
			continue
		}

		dbFileChanger := &allocation.UpdateFileChanger{}

		err := dbFileChanger.Unmarshal(c.Input)
		if err != nil {
			logging.Logger.Error("reloadChange", zap.Error(err))
		}

		// reload uploaded size from db, it was chunk size from client
		cmd.fileChanger.ThumbnailFilename = dbFileChanger.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = dbFileChanger.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = dbFileChanger.ThumbnailHash
		cmd.fileChanger.IsFinal = dbFileChanger.IsFinal
		return
	}
}

// UpdateChange add UpdateFileChanger in db
func (cmd *UpdateFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	for _, c := range connectionObj.Changes {
		filePath, _ := c.GetOrParseAffectedFilePath()
		if c.Operation != sdkConst.FileOperationUpdate || cmd.fileChanger.Path != filePath {
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
