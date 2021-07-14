package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// DeleteFileCommand command for deleting file
type DeleteFileCommand struct {
	exisitingFileRef *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
}

// IsAuthorized validate request.
func (cmd *DeleteFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.RepairerID != clientID {
		return common.NewError("invalid_operation", "Operation needs to be performed by the owner or the payer of the allocation")
	}

	path := req.FormValue("path")
	if len(path) == 0 {
		return common.NewError("invalid_parameters", "Invalid path")
	}
	cmd.exisitingFileRef, _ = reference.GetReference(ctx, allocationObj.ID, path)

	if cmd.exisitingFileRef == nil {
		common.NewError("invalid_file", "File does not exist at path")
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
	cmd.allocationChange.Operation = allocation.DELETE_OPERATION

	connectionObj.Size += cmd.allocationChange.Size

	return result, nil

}

// ProcessThumbnail no thumbnail should be processed for delete. A deffered delete command has been added on ProcessContent
func (cmd *DeleteFileCommand) ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error {
	//DO NOTHING
	return nil
}
