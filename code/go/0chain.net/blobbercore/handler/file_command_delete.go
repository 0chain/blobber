package handler

import (
	"context"
	"net/http"

	"0chain.net/blobbercore/allocation"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
)

// DeleteFileCommand command for deleting file
type DeleteFileCommand struct {
	exisitingFileRef *reference.Ref
	changeProcessor  *allocation.DeleteFileChange
	allocationChange *allocation.AllocationChange
}

// IsAuthorized validate request.
func (cmd *DeleteFileCommand) IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error {
	if allocationObj.OwnerID != clientID && allocationObj.PayerID != clientID {
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
func (cmd *DeleteFileCommand) UpdateChange(connectionObj *allocation.AllocationChangeCollector) {
	connectionObj.AddChange(cmd.allocationChange, cmd.changeProcessor)
}

// ProcessContent flush file to FileStorage
func (cmd *DeleteFileCommand) ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (UploadResult, error) {

	deleteSize := cmd.exisitingFileRef.Size

	cmd.changeProcessor = &allocation.DeleteFileChange{ConnectionID: connectionObj.ConnectionID,
		AllocationID: connectionObj.AllocationID, Name: cmd.exisitingFileRef.Name,
		Hash: cmd.exisitingFileRef.Hash, Path: cmd.exisitingFileRef.Path, Size: deleteSize}

	result := UploadResult{}
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
