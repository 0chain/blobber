package allocation

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/common/core/util/wmpt"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

// swagger:model UploadFileChanger
// UploadFileChanger file change processor for continuous upload in INIT/APPEND/FINALIZE
type UploadFileChanger struct {
	BaseFileChanger
}

// ApplyChange update references, and create a new FileRef
func (nf *UploadFileChanger) applyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {

	if !nf.IsFinal {
		return rootRef, nil
	}

	totalRefs, err := reference.CountRefs(ctx, nf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	fields, err := common.GetPathFields(filepath.Dir(nf.Path))
	if err != nil {
		return nil, err
	}
	if rootRef.CreatedAt == 0 {
		rootRef.CreatedAt = ts
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true

	dirRef := rootRef
	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				if child.Type != reference.DIRECTORY {
					return nil, common.NewError("invalid_reference_path", "Reference path has invalid ref type")
				}
				dirRef = child
				dirRef.UpdatedAt = ts
				dirRef.HashToBeComputed = true
				found = true
			}
		}

		if len(dirRef.Children) >= config.Configuration.MaxObjectsInDir {
			return nil, common.NewErrorf("max_objects_in_dir_reached",
				"maximum objects in directory %s reached: %v", dirRef.Path, config.Configuration.MaxObjectsInDir)
		}

		if !found {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = dirRef.AllocationID
			newRef.Path = "/" + strings.Join(fields[:i+1], "/")
			fileID, ok := fileIDMeta[newRef.Path]
			if !ok || fileID == "" {
				return nil, common.NewError("invalid_parameter",
					fmt.Sprintf("file path %s has no entry in fileID meta", newRef.Path))
			}
			newRef.FileID = fileID
			newRef.ParentPath = "/" + strings.Join(fields[:i], "/")
			newRef.Name = fields[i]
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			newRef.HashToBeComputed = true

			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	for _, child := range dirRef.Children {
		if child.Name == nf.Filename {
			return nil, common.NewError("duplicate_file", "File already exists")
		}
	}

	newFile := &reference.Ref{
		ActualFileHash:          nf.ActualHash,
		ActualFileHashSignature: nf.ActualFileHashSignature,
		ActualFileSize:          nf.ActualSize,
		AllocationID:            dirRef.AllocationID,
		ValidationRoot:          nf.ValidationRoot,
		ValidationRootSignature: nf.ValidationRootSignature,
		CustomMeta:              nf.CustomMeta,
		FixedMerkleRoot:         nf.FixedMerkleRoot,
		Name:                    nf.Filename,
		Path:                    nf.Path,
		ParentPath:              dirRef.Path,
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
		HashToBeComputed:        true,
		IsPrecommit:             true,
		FilestoreVersion:        filestore.VERSION,
	}

	fileID, ok := fileIDMeta[newFile.Path]
	if !ok || fileID == "" {
		return nil, common.NewError("invalid_parameter",
			fmt.Sprintf("file path %s has no entry in fileID meta", newFile.Path))
	}
	newFile.FileID = fileID

	dirRef.AddChild(newFile)

	return rootRef, nil
}

func (nf *UploadFileChanger) ApplyChangeV2(ctx context.Context, allocationRoot, clientPubKey string, numFiles *atomic.Int32, ts common.Timestamp, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) (int64, error) {
	if nf.AllocationID == "" {
		return 0, common.NewError("invalid_allocation_id", "Allocation ID is empty")
	}
	if !nf.IsFinal {
		return 0, nil
	}

	//find if ref exists
	var refResult struct {
		ID int64
	}

	nf.LookupHash = reference.GetReferenceLookup(nf.AllocationID, nf.Path)
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		return tx.Model(&reference.Ref{}).Select("id").Where("lookup_hash = ?", nf.LookupHash).Take(&refResult).Error
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}

	if refResult.ID > 0 {
		return 0, common.NewError("duplicate_file", "File already exists")
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
		ParentPath:              filepath.Dir(nf.Path),
		LookupHash:              nf.LookupHash,
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
		NumUpdates:              1,
	}
	nf.storageVersion = 1
	newFile.FileMetaHash = encryption.Hash(newFile.GetFileMetaHashDataV2())
	//create parent dir if it doesn't exist
	_, err = reference.Mkdir(ctx, nf.AllocationID, newFile.ParentPath, allocationRoot, ts, numFiles, collector)
	if err != nil {
		return 0, err
	}
	collector.CreateRefRecord(newFile)
	numFiles.Add(1)
	decodedKey, _ := hex.DecodeString(newFile.LookupHash)
	decodedValue, _ := hex.DecodeString(newFile.FileMetaHash)
	err = trie.Update(decodedKey, decodedValue, uint64(newFile.NumBlocks))
	return newFile.Size, err
}

// Marshal marshal and change to persistent to postgres
func (nf *UploadFileChanger) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

// Unmarshal reload and unmarshal change from allocation_changes.input on postgres
func (nf *UploadFileChanger) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}

func (nf *UploadFileChanger) GetPath() []string {
	return []string{nf.Path}
}
