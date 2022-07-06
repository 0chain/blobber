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
	return nil
}

func (rf *RenameFileChange) ApplyChange(ctx context.Context, change *AllocationChange,
	allocationRoot string, ts common.Timestamp) (*reference.Ref, error) {

	newPath := filepath.Join(filepath.Dir(rf.Path), rf.NewName)
	isFilePresent, err := reference.IsRefExist(ctx, rf.AllocationID, newPath)
	if err != nil {
		Logger.Info("invalid_reference_path", zap.Error(err))
	}

	if isFilePresent {
		return nil, common.NewError("invalid_reference_path", "file already exists")
	}

	affectedRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.Path)
	if err != nil {
		return nil, err
	}

	affectedRef.HashToBeComputed = true
	affectedRef.Name = rf.NewName
	affectedRef.Path = newPath
	if affectedRef.Type == reference.FILE {
		stats.FileUpdated(ctx, affectedRef.ID)
	}

	rf.processChildren(ctx, affectedRef)

	fields, err := common.GetPathFields(rf.Path)
	if err != nil {
		return nil, err
	}

	rootRef, err := reference.GetReferencePath(ctx, rf.AllocationID, rf.Path)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true
	dirRef := rootRef
	parentRef := rootRef

	var index int
	for i := 0; i < len(fields); i++ {
		found := false
		for idx, child := range dirRef.Children {
			if child.Name == fields[i] {
				parentRef = dirRef
				dirRef = child
				dirRef.UpdatedAt = ts
				dirRef.HashToBeComputed = true
				found = true
				index = idx
				break
			}
		}

		if !found {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}

	parentRef.RemoveChild(index)
	parentRef.AddChild(affectedRef)
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
