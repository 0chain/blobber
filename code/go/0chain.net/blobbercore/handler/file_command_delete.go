package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// FileCommandDelete command for deleting file
type FileCommandDelete struct {
	existingFileRef  *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
}

// IsValidated validate request.
func (cmd *FileCommandDelete) IsValidated(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	path := ctx.Value(constants.ContextKeyObjectPath).(string)
	if path == "" {
		return common.NewError("invalid_parameters", "Invalid path")
	}
	var err error
	cmd.existingFileRef, err = reference.GetLimitedRefFieldsByPath(ctx, allocationObj.ID, path, []string{"path", "name", "size", "hash", "merkle_root"})
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return common.ErrFileWasDeleted
		}
		return common.NewError("bad_db_operation", err.Error())
	}
	return nil
}

// UpdateChange add DeleteFileChange in db
func (cmd *FileCommandDelete) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}

// ProcessContent flush file to FileStorage
func (cmd *FileCommandDelete) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	deleteSize := cmd.existingFileRef.Size

	cmd.changeProcessor = &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, Name: cmd.existingFileRef.Name,
		Hash: cmd.existingFileRef.Hash, Path: cmd.existingFileRef.Path, Size: deleteSize}

	result := blobberhttp.UploadResult{}
	result.Filename = cmd.existingFileRef.Name
	result.Hash = cmd.existingFileRef.Hash
	result.MerkleRoot = cmd.existingFileRef.MerkleRoot
	result.Size = cmd.existingFileRef.Size

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ConnectionID
	cmd.allocationChange.Size = 0 - deleteSize
	cmd.allocationChange.Operation = constants.FileOperationDelete

	connectionObj.Size += cmd.allocationChange.Size

	return result, nil
}

// ProcessThumbnail no thumbnail should be processed for delete. A deffered delete command has been added on ProcessContent
func (cmd *FileCommandDelete) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	//DO NOTHING
	return nil
}
