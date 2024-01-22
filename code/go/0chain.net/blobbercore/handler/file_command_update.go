package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	sdkConst "github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
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

	if cmd.fileChanger.Size > config.StorageSCConfig.MaxFileSize {
		return common.NewError("max_file_size",
			fmt.Sprintf("file size %d should not be greater than %d", cmd.fileChanger.Size, config.StorageSCConfig.MaxFileSize))
	}

	if cmd.fileChanger.ConnectionID == "" {
		return common.NewError("invalid_connection", "Invalid connection id")
	}

	cmd.fileChanger.PathHash = encryption.Hash(cmd.fileChanger.Path)

	if cmd.fileChanger.ChunkSize <= 0 {
		cmd.fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}

	cmd.existingFileRef = allocation.GetExistingRef(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash)
	if cmd.existingFileRef == nil {
		cmd.existingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, cmd.fileChanger.Path)
		if cmd.existingFileRef == nil {
			return common.NewError("invalid_file_update", "File at path does not exist for update")
		}
		allocation.SaveExistingRef(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash, cmd.existingFileRef) //nolint:errcheck
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
	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *UpdateFileCommand) ProcessContent(allocationObj *allocation.Allocation) (allocation.UploadResult, error) {
	result := allocation.UploadResult{}

	result.Filename = cmd.fileChanger.Filename
	defer cmd.contentFile.Close()
	if cmd.fileChanger.IsFinal {
		result.UpdateChange = true
		cmd.reloadChange()
	}

	if cmd.fileChanger.Size == 0 {
		return result, common.NewError("invalid_parameters", "Invalid parameters. Size cannot be zero")
	}

	filePathHash := cmd.fileChanger.PathHash
	connID := cmd.fileChanger.ConnectionID

	fileInputData := &filestore.FileInputData{
		Name:         cmd.fileChanger.Filename,
		Path:         cmd.fileChanger.Path,
		UploadOffset: cmd.fileChanger.UploadOffset,
		IsFinal:      cmd.fileChanger.IsFinal,
		FilePathHash: filePathHash,
		Size:         cmd.fileChanger.Size,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connID, fileInputData, cmd.contentFile)
	if err != nil {
		return result, common.NewError("upload_error", "Failed to upload the file. "+err.Error())
	}

	result.ValidationRoot = fileOutputData.ValidationRoot
	result.FixedMerkleRoot = fileOutputData.FixedMerkleRoot
	result.Size = fileOutputData.Size

	allocationSize := allocation.GetConnectionObjSize(connID)

	cmd.fileChanger.AllocationID = allocationObj.ID

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connID
	cmd.allocationChange.Size = cmd.fileChanger.Size - cmd.existingFileRef.Size
	cmd.allocationChange.Operation = sdkConst.FileOperationUpdate

	if cmd.fileChanger.IsFinal {
		allocation.UpdateConnectionObjSize(connID, -cmd.existingFileRef.Size)
	}

	if fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
		allocation.UpdateConnectionObjSize(connID, fileOutputData.Size)
		saveChange, err := allocation.SaveFileChange(connID, cmd.fileChanger.PathHash, cmd.fileChanger.Filename, cmd, fileOutputData.Size, cmd.fileChanger.Size)
		if err != nil {
			return result, err
		}
		if saveChange {
			result.UpdateChange = false
		}
	}

	if allocationObj.BlobberSizeUsed+(allocationSize-cmd.existingFileRef.Size) > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	if cmd.thumbFile != nil {
		err = cmd.ProcessThumbnail(allocationObj)
		if err != nil {
			return result, err
		}
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
	return common.ErrNoThumbnail
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
	cmd.fileChanger.AllocationID = connectionObj.AllocationID
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

func (cmd *UpdateFileCommand) GetNumBlocks() int64 {
	if cmd.fileChanger.IsFinal {
		return int64(math.Ceil(float64(cmd.fileChanger.Size*1.0) / float64(cmd.fileChanger.ChunkSize)))
	}
	return 0
}
