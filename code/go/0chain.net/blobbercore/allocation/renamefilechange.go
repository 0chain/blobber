package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

type RenameFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	Path         string `json:"path"`
	NewName      string `json:"new_name"`
	Name         string `json:"name"`
}

func (rf *RenameFileChange) DeleteTempFile() error {
	return nil
}

func (rf *RenameFileChange) applyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	if rf.Path == "/" {
		return nil, common.NewError("invalid_operation", "cannot rename root path")
	}

	newPath := filepath.Join(filepath.Dir(rf.Path), rf.NewName)
	isFilePresent, err := reference.IsRefExist(ctx, rf.AllocationID, newPath)
	if err != nil {
		logging.Logger.Info("invalid_reference_path", zap.Error(err))
	}

	if isFilePresent {
		return nil, common.NewError("invalid_reference_path", "file already exists")
	}

	affectedRef, err := rootRef.GetSrcPath(rf.Path)
	if err != nil {
		return nil, err
	}
	affectedRef.HashToBeComputed = true
	affectedRef.Name = rf.NewName
	affectedRef.Path = newPath
	affectedRef.UpdatedAt = ts
	if affectedRef.Type == reference.FILE {
		affectedRef.IsPrecommit = true
	} else {
		rf.processChildren(ctx, affectedRef, ts)
	}

	parentPath := filepath.Dir(rf.Path)
	fields, err := common.GetPathFields(parentPath)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true
	dirRef := rootRef

	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				dirRef = child
				dirRef.UpdatedAt = ts
				dirRef.HashToBeComputed = true
				found = true
				break
			}
		}

		if !found {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}

	found := false
	for i, child := range dirRef.Children {
		if child.Path == rf.Path {
			dirRef.RemoveChild(i)
			dirRef.AddChild(affectedRef)
			found = true
			break
		}
	}
	if !found {
		return nil, common.NewError("file_not_found", "File to rename not found in blobber")
	}

	return rootRef, nil
}

func (rf *RenameFileChange) processChildren(ctx context.Context, curRef *reference.Ref, ts common.Timestamp) {
	for _, childRef := range curRef.Children {
		childRef.UpdatedAt = ts
		childRef.HashToBeComputed = true
		newPath := filepath.Join(curRef.Path, childRef.Name)
		childRef.UpdatePath(newPath, curRef.Path)
		if childRef.Type == reference.FILE {
			childRef.IsPrecommit = true
		}
		if childRef.Type == reference.DIRECTORY {
			rf.processChildren(ctx, childRef, ts)
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

func (rf *RenameFileChange) GetPath() []string {

	return []string{rf.Path}
}
