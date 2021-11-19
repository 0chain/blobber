package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

type RenameFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	Path         string `json:"path"`
	NewName      string `json:"new_name"`
}

func (rf *RenameFileChange) DeleteTempFile() error {
	return OperationNotApplicable
}

func (rf *RenameFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	isFilePresent, _ := reference.PathExists(ctx, rf.AllocationID, rf.NewName)
	if isFilePresent {
		return nil, common.NewError("invalid_reference_path", "file already exists")
	}

	affectedRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.Path)
	if err != nil {
		return nil, err
	}

	path, _ := filepath.Split(affectedRef.Path)
	path = filepath.Clean(path)
	affectedRef.Name = rf.NewName
	newPath := filepath.Join(path, rf.NewName)
	affectedRef.UpdatePath(newPath, path)
	if affectedRef.Type == reference.FILE {
		stats.FileUpdated(ctx, affectedRef.ID)
	}

	rf.processChildren(ctx, affectedRef)

	path, _ = filepath.Split(rf.Path)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath(ctx, rf.AllocationID, rf.Path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	treelevel := 0
	for treelevel < len(tSubDirs) {
		found := false
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY && treelevel < len(tSubDirs) {
				if child.Name == tSubDirs[treelevel] {
					dirRef = child
					found = true
					break
				}
			}
		}
		if found {
			treelevel++
		} else {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}

	if len(dirRef.Children) == 0 {
		Logger.Error("no files in root folder", zap.Any("change", rf))
		return nil, common.NewError("file_not_found", "No files in root folder")
	}

	idx := -1
	for i, child := range dirRef.Children {
		if child.Path == rf.Path {
			idx = i
			break
		}
	}
	if idx < 0 {
		Logger.Error("error in file rename", zap.Any("change", rf))
		return nil, common.NewError("file_not_found", "File to rename not found in blobber")
	}
	//dirRef.Children[idx] = affectedRef
	dirRef.RemoveChild(idx)
	dirRef.AddChild(affectedRef)
	_, err = rootRef.CalculateHash(ctx, true)

	return rootRef, err
}

func (rf *RenameFileChange) processChildren(ctx context.Context, curRef *reference.Ref) {
	for _, childRef := range curRef.Children {
		newPath := filepath.Join(curRef.Path, childRef.Name)
		childRef.UpdatePath(newPath, curRef.Path)
		if childRef.Type == reference.FILE {
			stats.FileUpdated(ctx, childRef.ID)
		}
		if childRef.Type == reference.DIRECTORY {
			rf.processChildren(ctx, childRef)
		}
	}
}

func (rf *RenameFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *RenameFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *RenameFileChange) CommitToFileStore(ctx context.Context) error {
	return nil
}
