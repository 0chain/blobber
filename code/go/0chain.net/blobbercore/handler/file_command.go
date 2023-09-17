package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

// FileCommand execute command for a file operation

// createFileCommand create file command for UPLOAD,UPDATE and DELETE
func createFileCommand(req *http.Request) allocation.FileCommand {
	switch req.Method {
	case http.MethodPost:
		return &UploadFileCommand{}
	case http.MethodPut:
		return &UpdateFileCommand{}
	case http.MethodDelete:
		return &DeleteFileCommand{}

	default:
		return &UploadFileCommand{}
	}
}

// validateParentPathType validates against any parent path not being directory.
func validateParentPathType(ctx context.Context, allocationID, fPath string) error {
	paths, err := common.GetParentPaths(fPath)
	if err != nil {
		return err
	}

	refs, err := reference.GetRefsTypeFromPaths(ctx, allocationID, paths)
	if err != nil {
		logging.Logger.Error(err.Error())
		return common.NewError("database_error", "Got error while getting parent refs")
	}

	for _, ref := range refs {
		if ref == nil {
			continue
		}
		if ref.Type == reference.FILE {
			return common.NewError("invalid_path", fmt.Sprintf("parent path %v is of file type", ref.Path))
		}
	}
	return nil
}
