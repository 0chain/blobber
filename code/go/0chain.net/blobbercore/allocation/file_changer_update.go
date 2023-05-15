package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

type UpdateFileChanger struct {
	deleteHash map[string]bool
	BaseFileChanger
}

func (nf *UpdateFileChanger) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	path := filepath.Clean(nf.Path)
	fields, err := common.GetPathFields(path)
	if err != nil {
		return nil, err
	}

	rootRef.HashToBeComputed = true
	rootRef.UpdatedAt = ts
	dirRef := rootRef

	var fileRef *reference.Ref
	var fileRefFound bool
	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY {
				if child.Name == fields[i] {
					dirRef = child
					dirRef.HashToBeComputed = true
					dirRef.UpdatedAt = ts
					found = true
				}
			} else {
				if child.Type == reference.FILE {
					if child.Name == fields[i] {
						fileRef = child
						fileRef.UpdatedAt = ts
						found = true
						fileRefFound = true
					}
				}
			}
		}

		if !found {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}

	if !fileRefFound {
		return nil, common.NewError("invalid_reference_path", "File to update not found in blobber")
	}

	fileRef.HashToBeComputed = true
	nf.deleteHash = make(map[string]bool)
	if fileRef.ThumbnailHash != "" && fileRef.ThumbnailHash != nf.ThumbnailHash {
		nf.deleteHash[fileRef.ThumbnailHash] = true
	}
	if fileRef.ValidationRoot != "" && fileRef.ValidationRoot != nf.ValidationRoot {
		nf.deleteHash[fileRef.ValidationRoot] = true
	}
	fileRef.ActualFileHash = nf.ActualHash
	fileRef.ActualFileHashSignature = nf.ActualFileHashSignature
	fileRef.ActualFileSize = nf.ActualSize
	fileRef.MimeType = nf.MimeType
	fileRef.ValidationRootSignature = nf.ValidationRootSignature
	fileRef.ValidationRoot = nf.ValidationRoot
	fileRef.CustomMeta = nf.CustomMeta
	fileRef.FixedMerkleRoot = nf.FixedMerkleRoot
	fileRef.AllocationRoot = allocationRoot
	fileRef.Size = nf.Size
	fileRef.ThumbnailHash = nf.ThumbnailHash
	fileRef.ThumbnailSize = nf.ThumbnailSize
	fileRef.ActualThumbnailHash = nf.ActualThumbnailHash
	fileRef.ActualThumbnailSize = nf.ActualThumbnailSize
	fileRef.EncryptedKey = nf.EncryptedKey
	fileRef.ChunkSize = nf.ChunkSize

	// _, err = rootRef.CalculateHash(ctx, true)
	// if err != nil {
	// 	return nil, err
	// }

	stats.FileUpdated(ctx, fileRef.ID)

	return rootRef, err
}

func (nf *UpdateFileChanger) CommitToFileStore(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	for hash := range nf.deleteHash {
		var count int64
		err := db.Table((&reference.Ref{}).TableName()).
			Where(
				db.Where(&reference.Ref{ThumbnailHash: hash}).
					Or(&reference.Ref{ValidationRoot: hash})).
			Where(&reference.Ref{AllocationID: nf.AllocationID}).
			Count(&count).Error

		if err == nil && count == 0 {
			logging.Logger.Info("Deleting content file", zap.String("validation_root", hash))
			if err := filestore.GetFileStore().DeleteFile(nf.AllocationID, hash); err != nil {
				logging.Logger.Error("FileStore_DeleteFile", zap.String("allocation_id", nf.AllocationID), zap.Error(err))
			}
		}
	}

	return nf.BaseFileChanger.CommitToFileStore(ctx)
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

func (nf *UpdateFileChanger) GetPath() []string {
	return []string{nf.Path}
}
