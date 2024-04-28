package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type UpdateFileChanger struct {
	deleteHash map[string]int
	BaseFileChanger
}

type HashType int

const (
	THUMBNAIL HashType = iota
	CONTENT
)

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
	nf.deleteHash = make(map[string]int)

	if fileRef.ValidationRoot != "" && fileRef.ValidationRoot != nf.ValidationRoot {
		nf.deleteHash[fileRef.ValidationRoot] = fileRef.FilestoreVersion
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
	fileRef.EncryptedKeyPoint = nf.EncryptedKeyPoint
	fileRef.ChunkSize = nf.ChunkSize
	fileRef.IsPrecommit = true
	fileRef.FilestoreVersion = filestore.VERSION

	return rootRef, nil
}

func (nf *UpdateFileChanger) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	db := datastore.GetStore().GetTransaction(ctx)
	for hash, version := range nf.deleteHash {
		var count int64
		mut.Lock()
		err := db.Table((&reference.Ref{}).TableName()).
			Where(&reference.Ref{ValidationRoot: hash}).
			Where(&reference.Ref{AllocationID: nf.AllocationID}).
			Count(&count).Error
		mut.Unlock()
		if err == nil && count == 0 {
			logging.Logger.Info("Deleting content file", zap.String("validation_root", hash))
			if err := filestore.GetFileStore().DeleteFile(nf.AllocationID, hash, version); err != nil {
				logging.Logger.Error("FileStore_DeleteFile", zap.String("allocation_id", nf.AllocationID), zap.Error(err))
			}
		}
	}

	return nf.BaseFileChanger.CommitToFileStore(ctx, mut)
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
