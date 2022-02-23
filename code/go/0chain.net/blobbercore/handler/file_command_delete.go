package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/gosdk/constants"
)

// FileCommandDelete command for deleting file
type FileCommandDelete struct {
	exisitingFileRef *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
}

// IsAuthorized validate request.
func (cmd *FileCommandDelete) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	path := req.FormValue("path")
	if path == "" {
		return common.NewError("invalid_parameters", "Invalid path")
	}
	cmd.exisitingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, path)

	if cmd.exisitingFileRef == nil {
		return common.NewErrorfWithStatusCode(204, "invalid_file", "File does not exist at path")
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
