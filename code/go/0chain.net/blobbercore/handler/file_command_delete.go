package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

// DeleteFileCommand command for deleting file
type DeleteFileCommand struct {
	existingFileRef  *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
	path             string
	connectionID     string
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

	connectionID, ok := common.GetField(req, "connection_id")
	if !ok {
		return common.NewError("invalid_parameters", "Invalid connection id passed")
	}
	cmd.connectionID = connectionID
	var err error
	pathHash := encryption.Hash(path)
	err = allocation.GetError(connectionID, pathHash)
	if err != nil {
		return err
	}
	lookUpHash := reference.GetReferenceLookup(allocationObj.ID, path)
	cmd.existingFileRef, err = reference.GetLimitedRefFieldsByLookupHashWith(ctx, allocationObj.ID, lookUpHash, []string{"path", "name", "size", "hash", "fixed_merkle_root"})
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return common.ErrFileWasDeleted
		}
		return common.NewError("bad_db_operation", err.Error())
	}
	allocation.CreateConnectionChange(connectionID, pathHash)

	return allocation.SetFinalized(connectionID, pathHash, cmd)
}

// UpdateChange add DeleteFileChange in db
func (cmd *DeleteFileCommand) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}

// ProcessContent flush file to FileStorage
func (cmd *DeleteFileCommand) ProcessContent(allocationObj *allocation.Allocation) (allocation.UploadResult, error) {
	deleteSize := cmd.existingFileRef.Size
	connectionID := cmd.connectionID
	cmd.changeProcessor = &allocation.DeleteFileChange{ConnectionID: connectionID,
		AllocationID: allocationObj.ID, Name: cmd.existingFileRef.Name,
		Hash: cmd.existingFileRef.Hash, Path: cmd.existingFileRef.Path, Size: deleteSize}

	result := allocation.UploadResult{}
	result.Filename = cmd.existingFileRef.Name
	result.ValidationRoot = cmd.existingFileRef.ValidationRoot
	result.FixedMerkleRoot = cmd.existingFileRef.FixedMerkleRoot
	result.Size = cmd.existingFileRef.Size
	result.IsFinal = true

	cmd.allocationChange = &allocation.AllocationChange{}
	cmd.allocationChange.ConnectionID = connectionID
	cmd.allocationChange.Size = 0 - deleteSize
	cmd.allocationChange.Operation = constants.FileOperationDelete

	allocation.UpdateConnectionObjSize(connectionID, cmd.allocationChange.Size)

	return result, nil
}

// ProcessThumbnail no thumbnail should be processed for delete. A deffered delete command has been added on ProcessContent
func (cmd *DeleteFileCommand) ProcessThumbnail(allocationObj *allocation.Allocation) error {
	//DO NOTHING
	return nil
}
