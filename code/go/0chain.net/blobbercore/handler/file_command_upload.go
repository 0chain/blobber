package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/fileref"
	"go.uber.org/zap"
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

	if fileChanger.Path == "/" {
		return common.NewError("invalid_path", "Invalid path. Cannot upload to root directory")
	}

	if !filepath.IsAbs(fileChanger.Path) {
		return common.NewError("invalid_path", fmt.Sprintf("%v is not absolute path", fileChanger.Path))
	}

	isExist, err := reference.IsRefExist(ctx, allocationObj.ID, fileChanger.Path)

	if err != nil {
		logging.Logger.Error(err.Error())
		return common.NewError("database_error", "Got db error while getting ref")
	}

	if isExist {
		msg := fmt.Sprintf("File at path :%s: already exists", fileChanger.Path)
		return common.NewError("duplicate_file", msg)
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

	if fileChanger.ChunkSize <= 0 {
		fileChanger.ChunkSize = fileref.CHUNK_SIZE
	}

	cmd.fileChanger = fileChanger
	return nil
}

// ProcessContent flush file to FileStorage
func (cmd *UploadFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	result := blobberhttp.UploadResult{}

	origfile, _, err := req.FormFile(UploadFile)
	if err != nil {
		return result, common.NewError("invalid_parameters", "Error Reading multi parts for file."+err.Error())
	}
	defer origfile.Close()

	cmd.reloadChange(connectionObj)

	var hasher *filestore.CommitHasher
	filePathHash := encryption.Hash(cmd.fileChanger.Path)
	if cmd.fileChanger.Size == 0 {
		return result, common.NewError("invalid_parameters", "Invalid parameters. Size cannot be zero")
	}
	if cmd.fileChanger.UploadOffset == 0 {
		hasher = filestore.GetNewCommitHasher(cmd.fileChanger.Size)
		allocation.UpdateConnectionObjWithHasher(connectionObj.ID, filePathHash, hasher)
	} else {
		hasher = allocation.GetHasher(connectionObj.ID, filePathHash)
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

	result.Filename = cmd.fileChanger.Filename
	result.ValidationRoot = fileOutputData.ValidationRoot
	result.Size = fileOutputData.Size

	allocationSize := connectionObj.Size

	// only update connection size when the chunk is uploaded.
	if fileOutputData.ChunkUploaded {
		allocationSize += fileOutputData.Size
		allocation.UpdateConnectionObjSize(connectionObj.ID, fileOutputData.Size)
	}

	if allocationObj.BlobberSizeUsed+allocationSize > allocationObj.BlobberSize {
		return result, common.NewError("max_allocation_size", "Max size reached for the allocation with this blobber")
	}

	cmd.fileChanger.AllocationID = allocationObj.ID
	// cmd.fileChanger.Size += fileOutputData.Size

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ID
	cmd.allocationChange.Size = cmd.fileChanger.Size
	cmd.allocationChange.Operation = constants.FileOperationInsert

	connectionObj.Size = allocationSize

	return result, nil
}

// ProcessThumbnail flush thumbnail file to FileStorage if it has.
func (cmd *UploadFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	thumbfile, thumbHeader, _ := req.FormFile(UploadThumbnailFile)

	if thumbHeader != nil {
		defer thumbfile.Close()

		thumbInputData := &filestore.FileInputData{Name: thumbHeader.Filename, Path: cmd.fileChanger.Path}
		thumbOutputData, err := filestore.GetFileStore().WriteFile(allocationObj.ID, connectionObj.ID, thumbInputData, thumbfile)
		if err != nil {
			return common.NewError("upload_error", "Failed to upload the thumbnail. "+err.Error())
		}

		cmd.fileChanger.ThumbnailSize = thumbOutputData.Size
		cmd.fileChanger.ThumbnailFilename = thumbInputData.Name
	}

	return nil
}

func (cmd *UploadFileCommand) reloadChange(connectionObj *allocation.AllocationChangeCollector) {
	for _, c := range connectionObj.Changes {
		filePath, _ := c.GetOrParseAffectedFilePath()
		if c.Operation != constants.FileOperationInsert || cmd.fileChanger.Path != filePath {
			continue
		}

		dbChangeProcessor := &allocation.UploadFileChanger{}

		err := dbChangeProcessor.Unmarshal(c.Input)
		if err != nil {
			logging.Logger.Error("reloadChange", zap.Error(err))
		}

		cmd.fileChanger.Size = dbChangeProcessor.Size
		cmd.fileChanger.ThumbnailFilename = dbChangeProcessor.ThumbnailFilename
		cmd.fileChanger.ThumbnailSize = dbChangeProcessor.ThumbnailSize
		cmd.fileChanger.ThumbnailHash = dbChangeProcessor.ThumbnailHash

		return
	}
}

// UpdateChange replace AddFileChange in db
func (cmd *UploadFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
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
