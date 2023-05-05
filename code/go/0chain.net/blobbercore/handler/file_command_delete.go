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

// DeleteFileCommand command for deleting file
type DeleteFileCommand struct {
	existingFileRef  *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
	path             string
}

func (cmd *DeleteFileCommand) GetExistingFileRef() *reference.Ref {
	return cmd.existingFileRef
}

func (cmd *DeleteFileCommand) GetPath() string {
	return cmd.path
}

// IsValidated validate request.
func (cmd *DeleteFileCommand) IsValidated(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	path, ok := common.GetField(req, "path")
	if !ok {
		return common.NewError("invalid_parameters", "Invalid path")
	}

	cmd.path = path

	var err error
	cmd.existingFileRef, err = reference.GetLimitedRefFieldsByPath(ctx, allocationObj.ID, path, []string{"path", "name", "size", "hash", "fixed_merkle_root"})
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return common.ErrFileWasDeleted
		}
		return common.NewError("bad_db_operation", err.Error())
	}
	return nil
}

// UpdateChange add DeleteFileChange in db
func (cmd *DeleteFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}

// ProcessContent flush file to FileStorage
func (cmd *DeleteFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	deleteSize := cmd.existingFileRef.Size

	cmd.changeProcessor = &allocation.DeleteFileChange{ConnectionID: connectionObj.ID,
		AllocationID: connectionObj.AllocationID, Name: cmd.existingFileRef.Name,
		Hash: cmd.existingFileRef.Hash, Path: cmd.existingFileRef.Path, Size: deleteSize}

	result := blobberhttp.UploadResult{}
	result.Filename = cmd.existingFileRef.Name
	result.ValidationRoot = cmd.existingFileRef.ValidationRoot
	result.FixedMerkleRoot = cmd.existingFileRef.FixedMerkleRoot
	result.Size = cmd.existingFileRef.Size

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionObj.ID
	cmd.allocationChange.Size = 0 - deleteSize
	cmd.allocationChange.Operation = constants.FileOperationDelete

	connectionObj.Size += cmd.allocationChange.Size
	allocation.UpdateConnectionObjSize(connectionObj.ID, cmd.allocationChange.Size)

	return result, nil
}

// ProcessThumbnail no thumbnail should be processed for delete. A deffered delete command has been added on ProcessContent
func (cmd *DeleteFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	//DO NOTHING
	return nil
}
