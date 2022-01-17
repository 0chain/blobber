package handler

import (
	"context"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/blobberhttp"
)

// FileCommand execute command for a file operation
type FileCommand interface {
	// IsAuthorized validate request, and try build ChangeProcesser instance
	IsAuthorized(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, clientID string) error

	// ProcessContent flush file to FileStorage
	ProcessContent(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) (blobberhttp.UploadResult, error)

	// ProcessThumbnail flush thumbnail file to FileStorage if it has.
	ProcessThumbnail(ctx context.Context, req *http.Request, allocationObj *allocation.Allocation, connectionObj *allocation.AllocationChangeCollector) error

	// UpdateChange update AllocationChangeProcessor. It will be president in db for committing transcation
	UpdateChange(ctx context.Context, connectionObj *allocation.AllocationChangeCollector) error
}

// createFileCommand create file command for UPLOAD,UPDATE and DELETE
func createFileCommand(req *http.Request) FileCommand {
	switch req.Method {
	case http.MethodPost:
		return &AddFileCommand{}
	case http.MethodPut:
		return &UpdateFileCommand{}
	case http.MethodDelete:
		return &FileCommandDelete{}

	default:
		return &AddFileCommand{}
	}
}
