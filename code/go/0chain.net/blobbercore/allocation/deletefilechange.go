package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
)

type DeleteFileChange struct {
	ConnectionID string      `json:"connection_id"`
	AllocationID string      `json:"allocation_id"`
	Name         string      `json:"name"`
	Path         string      `json:"path"`
	Size         int64       `json:"size"`
	Hash         string      `json:"hash"`
	DeleteToken  interface{} `json:"delete_token"`
}

func (nf *DeleteFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	path, _ := filepath.Split(nf.Path)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)
	rootRef, err := reference.GetReferencePath(ctx, nf.AllocationID, nf.Path)
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
	idx := -1
	for i, child := range dirRef.Children {
		if child.Type == reference.FILE && child.Hash == nf.Hash {
			idx = i
			reference.DeleteReference(ctx, child.ID, child.PathHash)
			break
		}
	}
	if idx < 0 {
		return nil, common.NewError("file_not_found", "File to delete not found in blobber")
	}
	//dirRef.Children = append(dirRef.Children[:idx], dirRef.Children[idx+1:]...)
	dirRef.RemoveChild(idx)
	rootRef.CalculateHash(ctx, true)
	return nil, nil
}

func (nf *DeleteFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nf *DeleteFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), nf)
	return err
}
