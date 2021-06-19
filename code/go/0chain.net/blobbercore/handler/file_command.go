package handler

import (
	"context"
	"net/http"

	"0chain.net/blobbercore/allocation"
)

// FileCommand execute command for a file operation
type FileCommand interface {
	// IsAuthorized validate request, and try build ChangeProcesser instance
	IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error

	// ProcessContent flush file to FileStorage
	ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (UploadResult, error)

	// ProcessThumbnail flush thumbnail file to FileStorage if it has.
	ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error

	// UpdateChange update AllocationChangeProcessor. It will be president in db for commiting transcation
	UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error
}

// createFileCommand create file command for INSERT,UPDATE and RESUME
func createFileCommand(req *http.Request) FileCommand {
	switch req.Method {
	case http.MethodPatch:
		return &ResumeFileCommand{}
	case http.MethodPut:
		return &UpdateFileCommand{}
	case http.MethodPost:
		return &InsertFileCommand{}
	case http.MethodDelete:
		return &DeleteFileCommand{}
	default:
		return &InsertFileCommand{}
	}
}
