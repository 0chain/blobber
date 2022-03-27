package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"
)

// FileCommandDelete command for deleting file
type FileCommandDelete struct {
	exisitingFileRef *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
}

// IsValidated validate request.
func (cmd *FileCommandDelete) IsValidated(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	path := req.FormValue("path")
	if path == "" {
		return common.NewError("invalid_parameters", "Invalid path")
	}

	fileRef, err := reference.GetReference(ctx, allocationObj.ID, path)
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return common.ErrFileWasDeleted
		}
		return common.NewError("bad_db_operation", err.Error())
	}

	cmd.exisitingFileRef = fileRef

	return nil
}

// UpdateChange add DeleteFileChange in db
func (cmd *FileCommandDelete) UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)

	return connectionObj.Save(ctx)
}

// ProcessContent flush file to FileStorage
func (cmd *FileCommandDelete) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error) {
	deleteSize := cmd.exisitingFileRef.Size

	cmd.changeProcessor = &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, Name: cmd.exisitingFileRef.Name,
		Hash: cmd.exisitingFileRef.Hash, Path: cmd.exisitingFileRef.Path, Size: deleteSize}

	result := blobberhttp.UploadResult{}
	result.Filename = cmd.exisitingFileRef.Name
	result.Hash = cmd.exisitingFileRef.Hash
	result.MerkleRoot = cmd.exisitingFileRef.MerkleRoot
	result.Size = cmd.exisitingFileRef.Size

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
