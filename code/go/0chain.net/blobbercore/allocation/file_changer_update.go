package allocation

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/common/core/util/wmpt"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
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

func (nf *UpdateFileChanger) ApplyChangeV2(ctx context.Context, allocationRoot, clientPubKey string, numFiles *atomic.Int32, ts common.Timestamp, hashSignature map[string]string, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) error {
	if nf.AllocationID == "" {
		return common.NewError("invalid_allocation_id", "Allocation ID is empty")
	}

	parentPath := filepath.Dir(nf.Path)
	nf.LookupHash = reference.GetReferenceLookup(nf.AllocationID, parentPath)

	//find if ref exists
	var refResult struct {
		ID           int64
		Type         string
		NumUpdates   int64  `gorm:"column:num_of_updates" json:"num_of_updates"`
		FileMetaHash string `gorm:"column:file_meta_hash" json:"file_meta_hash"`
	}

	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		return tx.Model(&reference.Ref{}).Select("id", "type", "num_of_updates").Where("lookup_hash = ?", nf.LookupHash).Take(&refResult).Error
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if refResult.ID == 0 {
		return common.NewError("file_not_found", "File not found")
	}

	if refResult.Type != reference.FILE {
		return common.NewError("invalid_reference_type", "Cannot update a directory")
	}

	newFile := &reference.Ref{
		ActualFileHash:          nf.ActualHash,
		ActualFileHashSignature: nf.ActualFileHashSignature,
		ActualFileSize:          nf.ActualSize,
		AllocationID:            nf.AllocationID,
		ValidationRoot:          nf.ValidationRoot,
		ValidationRootSignature: nf.ValidationRootSignature,
		CustomMeta:              nf.CustomMeta,
		FixedMerkleRoot:         nf.FixedMerkleRoot,
		Name:                    nf.Filename,
		Path:                    nf.Path,
		ParentPath:              parentPath,
		Type:                    reference.FILE,
		Size:                    nf.Size,
		MimeType:                nf.MimeType,
		AllocationRoot:          allocationRoot,
		ThumbnailHash:           nf.ThumbnailHash,
		ThumbnailSize:           nf.ThumbnailSize,
		ActualThumbnailHash:     nf.ActualThumbnailHash,
		ActualThumbnailSize:     nf.ActualThumbnailSize,
		EncryptedKey:            nf.EncryptedKey,
		EncryptedKeyPoint:       nf.EncryptedKeyPoint,
		ChunkSize:               nf.ChunkSize,
		CreatedAt:               ts,
		UpdatedAt:               ts,
		FilestoreVersion:        filestore.VERSION,
		PathLevel:               len(strings.Split(strings.TrimRight(nf.Path, "/"), "/")),
		NumBlocks:               int64(math.Ceil(float64(nf.Size*1.0) / float64(nf.ChunkSize))),
		NumUpdates:              refResult.NumUpdates + 1,
	}
	nf.storageVersion = 1
	newFile.FileMetaHash = encryption.Hash(newFile.GetFileHashDataV2())
	sig, ok := hashSignature[newFile.LookupHash]
	if !ok {
		return common.NewError("invalid_hash_signature", "Hash signature not found")
	}
	fileHash := encryption.Hash(newFile.GetFileHashDataV2())
	//verify signature
	verify, err := encryption.Verify(clientPubKey, sig, fileHash)
	if err != nil || !verify {
		return common.NewError("invalid_signature", "Signature is invalid")
	}
	newFile.Hash = sig
	deleteRecord := &reference.Ref{
		ID:         refResult.ID,
		LookupHash: newFile.LookupHash,
		Type:       refResult.Type,
	}
	collector.DeleteRefRecord(deleteRecord)
	collector.CreateRefRecord(newFile)
	decodedKey, _ := hex.DecodeString(newFile.LookupHash)
	decodedValue, _ := hex.DecodeString(newFile.FileMetaHash)
	err = trie.Update(decodedKey, nil, 0)
	if err != nil {
		return err
	}
	err = trie.Update(decodedKey, decodedValue, uint64(newFile.NumBlocks))
	return err
}

func (nf *UpdateFileChanger) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
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
