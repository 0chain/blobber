package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

type UpdateFileChanger struct {
	BaseFileChanger
}

func (nf *UpdateFileChanger) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
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
		if child.Type == reference.FILE && child.Path == nf.Path {
			idx = i
			break
		}
	}
	if idx < 0 {
		Logger.Error("error in file update", zap.Any("change", nf))
		return nil, common.NewError("file_not_found", "File to update not found in blobber")
	}
	existingRef := dirRef.Children[idx]
	existingRef.ActualFileHash = nf.ActualHash
	existingRef.ActualFileSize = nf.ActualSize
	existingRef.MimeType = nf.MimeType
	existingRef.ContentHash = nf.Hash
	existingRef.CustomMeta = nf.CustomMeta
	existingRef.MerkleRoot = nf.MerkleRoot
	existingRef.WriteMarker = allocationRoot
	existingRef.Size = nf.Size
	existingRef.ThumbnailHash = nf.ThumbnailHash
	existingRef.ThumbnailSize = nf.ThumbnailSize
	existingRef.ActualThumbnailHash = nf.ActualThumbnailHash
	existingRef.ActualThumbnailSize = nf.ActualThumbnailSize
	existingRef.EncryptedKey = nf.EncryptedKey
	existingRef.ChunkSize = nf.ChunkSize

	if err = existingRef.SetAttributes(&nf.Attributes); err != nil {
		return nil, common.NewErrorf("process_update_file_change",
			"setting file attributes: %v", err)
	}

	_, err = rootRef.CalculateHash(ctx, true)
	stats.FileUpdated(ctx, existingRef.ID)
	return rootRef, err
}

func (nf *UpdateFileChanger) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nf *UpdateFileChanger) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}
