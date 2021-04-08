package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/filestore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/common"
	. "0chain.net/core/logging"

	"go.uber.org/zap"
)

type DeleteFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	Hash         string `json:"hash"`
	ContentHash  map[string]bool
}

func (nf *DeleteFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	affectedRef, err := reference.GetObjectTree(ctx, nf.AllocationID, nf.Path)

	if err != nil {
		return nil, err
	}
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
		if child.Hash == nf.Hash && child.Hash == affectedRef.Hash {
			idx = i
			nf.ContentHash = make(map[string]bool)
			if err := reference.DeleteReference(ctx, child.ID, child.PathHash); err != nil {
				Logger.Error("DeleteReference", zap.Int64("ref_id", child.ID), zap.Error(err))
			}
			if child.Type == reference.FILE {
				nf.ContentHash[child.ThumbnailHash] = true
				nf.ContentHash[child.ContentHash] = true
			} else {
				nf.processChildren(ctx, affectedRef)
			}
			break
		}
	}
	if idx < 0 {
		return nil, common.NewError("file_not_found", "Object to delete not found in blobber")
	}

	dirRef.RemoveChild(idx)
	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}

	return nil, nil
}

func (nf *DeleteFileChange) processChildren(ctx context.Context, curRef *reference.Ref) {
	for _, childRef := range curRef.Children {
		if err := reference.DeleteReference(ctx, childRef.ID, childRef.PathHash); err != nil {
			Logger.Error("DeleteReference", zap.Int64("ref_id", childRef.ID), zap.Error(err))
		}
		if childRef.Type == reference.FILE {
			nf.ContentHash[childRef.ThumbnailHash] = true
			nf.ContentHash[childRef.ContentHash] = true
		} else if childRef.Type == reference.DIRECTORY {
			nf.processChildren(ctx, childRef)
		}
	}
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

func (nf *DeleteFileChange) DeleteTempFile() error {
	return OperationNotApplicable
}

func (nf *DeleteFileChange) CommitToFileStore(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	for contenthash := range nf.ContentHash {
		var count int64
		err := db.Table((&reference.Ref{}).TableName()).Where(&reference.Ref{ThumbnailHash: contenthash}).Or(&reference.Ref{ContentHash: contenthash}).Count(&count).Error
		if err == nil && count == 0 {
			Logger.Info("Deleting content file", zap.String("content_hash", contenthash))
			if err := filestore.GetFileStore().DeleteFile(nf.AllocationID, contenthash); err != nil {
				Logger.Error("FileStore_DeleteFile", zap.String("allocation_id", nf.AllocationID), zap.Error(err))
			}
		}
	}
	return nil
}
