package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
)

const (
	MaxThumbnailSize    = MB
	UploadMeta          = "uploadMeta"
	UploadFile          = "uploadFile"
	UploadThumbnailFile = "uploadThumbnailFile"
)

// UploadFileCommand command for resuming file
type UploadFileCommand struct {
	allocationChange *allocation.AllocationChange
	fileChanger      *allocation.UploadFileChanger
	contentFile      multipart.File
	thumbFile        multipart.File
	thumbHeader      *multipart.FileHeader
}

func (cmd *UploadFileCommand) GetExistingFileRef() *reference.Ref {
	return nil
}

func (cmd *UploadFileCommand) GetPath() string {
	if cmd.fileChanger == nil {
		return ""
	}
	return cmd.fileChanger.Path
}

// IsValidated validate request.
func (cmd *UploadFileCommand) IsValidated(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	fileChanger := &allocation.UploadFileChanger{}
	uploadMetaString := req.FormValue(UploadMeta)
	err := json.Unmarshal([]byte(uploadMetaString), fileChanger)
	if err != nil {
		return common.NewError("invalid_parameters",
			"Invalid parameters. Error parsing the meta data for upload."+err.Error())
	}

	if fileChanger.Size > config.StorageSCConfig.MaxFileSize {
		return common.NewError("max_file_size",
			fmt.Sprintf("file size %d should not be greater than %d", fileChanger.Size, config.StorageSCConfig.MaxFileSize))
	}

	if fileChanger.Path == "/" {
		return common.NewError("invalid_path", "Invalid path. Cannot upload to root directory")
	}

	if !filepath.IsAbs(fileChanger.Path) {
		return common.NewError("invalid_path", fmt.Sprintf("%v is not absolute path", fileChanger.Path))
	}

	if fileChanger.ConnectionID == "" {
		return common.NewError("invalid_connection", "Invalid connection id")
	}

	fileChanger.PathHash = encryption.Hash(fileChanger.Path)

	err = allocation.GetError(fileChanger.ConnectionID, fileChanger.PathHash)
	if err != nil {
		return err
	}

	if fileChanger.UploadOffset == 0 {
		isExist, err := reference.IsRefExist(ctx, allocationObj.ID, fileChanger.Path)

		if err != nil {
			logging.Logger.Error(err.Error())
			return common.NewError("database_error", "Got db error while getting ref")
		}

		if isExist {
			msg := fmt.Sprintf("File at path :%s: already exists", fileChanger.Path)
			return common.NewError("duplicate_file", msg)
		}
		allocation.CreateConnectionChange(fileChanger.ConnectionID, fileChanger.PathHash, allocationObj)
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

	if fileChanger.ChunkSize <= 0 {
		fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}

	origfile, _, err := req.FormFile(UploadFile)
	if err != nil {
		return common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	cmd.contentFile = origfile
	cmd.fileChanger = fileChanger
	logging.Logger.Info("UploadFileCommand.IsValidated")
	if fileChanger.IsFinal {
		return allocation.SetFinalized(fileChanger.ConnectionID, fileChanger.PathHash, cmd)
	}
	return allocation.SendCommand(fileChanger.ConnectionID, fileChanger.PathHash, cmd)
}

// ProcessContent flush file to FileStorage
func (cmd *UploadFileCommand) ProcessContent(allocationObj *allocation.Allocation) (allocation.UploadResult, error) {
	result := allocation.UploadResult{}
	defer cmd.contentFile.Close()
	if cmd.fileChanger.IsFinal {
		cmd.reloadChange()
	}
	connectionID := cmd.fileChanger.ConnectionID
	var hasher *filestore.CommitHasher
	if cmd.fileChanger.Size == 0 {
		return result, common.NewError("invalid_parameters", "Invalid parameters. Size cannot be zero")
	}
	if cmd.fileChanger.UploadOffset == 0 {
		result.UpdateChange = true
		hasher = filestore.GetNewCommitHasher(cmd.fileChanger.Size)
		allocation.UpdateConnectionObjWithHasher(connectionID, cmd.fileChanger.PathHash, hasher)
	} else {
		hasher = allocation.GetHasher(connectionID, cmd.fileChanger.PathHash)
		if hasher == nil {
			return result, common.NewError("invalid_parameters", "Error getting hasher for upload.")
		}
	}

	fileInputData := &filestore.FileInputData{
		Name: cmd.fileChanger.Filename,
		Path: cmd.fileChanger.Path,

		ChunkSize:    cmd.fileChanger.ChunkSize,
		UploadOffset: cmd.fileChanger.UploadOffset,
		IsFinal:      cmd.fileChanger.IsFinal,
		FilePathHash: cmd.fileChanger.PathHash,
		Hasher:       hasher,
	}
	fileOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connectionID, fileInputData, cmd.contentFile)
	if err != nil {
		logging.Logger.Error("UploadFileCommand.ProcessContent", zap.Error(err))
		return result, common.NewError("upload_error", "Failed to write file. "+err.Error())
	}

	if cmd.fileChanger.IsFinal {
		err = hasher.Finalize()
		if err != nil {
			return result, common.NewError("upload_error", "Failed to finalize the hasher. "+err.Error())
		}
		result.IsFinal = true
	}

	result.Filename = cmd.fileChanger.Filename
	result.ValidationRoot = fileOutputData.ValidationRoot
	result.Size = fileOutputData.Size

	allocationSize := allocation.GetConnectionObjSize(connectionID)

	// only update connection size when the chunk is uploaded.
	if fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
		allocation.UpdateConnectionObjSize(connectionID, fileOutputData.Size)
	}

	if allocationObj.BlobberSizeUsed+allocationSize > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.fileChanger.AllocationID = allocationObj.ID

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionID
	cmd.allocationChange.Size = cmd.fileChanger.Size
	cmd.allocationChange.Operation = constants.FileOperationInsert
	logging.Logger.Info("Chunk processed")
	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UploadFileCommand) ProcessThumbnail(allocationObj *allocation.Allocation) error {
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
		return allocation.SaveFileChanger(connectionID, &cmd.fileChanger.BaseFileChanger)
	}
	return common.ErrNoThumbnail
}

func (cmd *UploadFileCommand) reloadChange() {
	changer := allocation.GetFileChanger(cmd.fileChanger.ConnectionID, cmd.fileChanger.PathHash)
	if changer != nil {
		cmd.fileChanger.ThumbnailFilename = changer.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = changer.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = changer.ThumbnailHash
	}
}

// UpdateChange replace AddFileChange in db
func (cmd *UploadFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	cmd.fileChanger.AllocationID = connectionObj.AllocationID
	for _, c := range connectionObj.Changes {
		filePath, _ := c.GetOrParseAffectedFilePath()
		if c.Operation != constants.FileOperationInsert || cmd.fileChanger.Path != filePath {
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

func (cmd *UploadFileCommand) GetNumBlocks() int64 {
	if cmd.fileChanger.IsFinal {
		return int64(math.Ceil(float64(cmd.fileChanger.Size*1.0) / float64(cmd.fileChanger.ChunkSize)))
	}
	return 0
}
