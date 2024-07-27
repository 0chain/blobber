package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
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
func (nf *UploadFileChanger) applyChange(ctx context.Context,
	ts common.Timestamp, _ map[string]string, collector reference.QueryCollector) error {

	if nf.AllocationID == "" {
		return common.NewError("invalid_parameter", "allocation_id is required")
	}

	parentPath := filepath.Dir(nf.Path)
	nf.LookupHash = reference.GetReferenceLookup(nf.AllocationID, nf.Path)
	newFile := &reference.Ref{
		ActualFileHash:          nf.ActualHash,
		ActualFileHashSignature: nf.ActualFileHashSignature,
		ActualFileSize:          nf.ActualSize,
		AllocationID:            nf.AllocationID,
		CustomMeta:              nf.CustomMeta,
		Name:                    nf.Filename,
		Path:                    nf.Path,
		ParentPath:              parentPath,
		Type:                    reference.FILE,
		Size:                    nf.Size,
		MimeType:                nf.MimeType,
		ThumbnailHash:           nf.ThumbnailHash,
		ThumbnailSize:           nf.ThumbnailSize,
		ActualThumbnailHash:     nf.ActualThumbnailHash,
		ActualThumbnailSize:     nf.ActualThumbnailSize,
		EncryptedKey:            nf.EncryptedKey,
		EncryptedKeyPoint:       nf.EncryptedKeyPoint,
		ChunkSize:               nf.ChunkSize,
		CreatedAt:               ts,
		UpdatedAt:               ts,
		IsPrecommit:             true,
		LookupHash:              nf.LookupHash,
		DataHash:                nf.DataHash,
		DataHashSignature:       nf.DataHashSignature,
		PathLevel:               len(strings.Split(strings.TrimRight(nf.Path, "/"), "/")),
		FilestoreVersion:        filestore.VERSION,
	}
	newFile.FileMetaHash = encryption.Hash(newFile.GetFileMetaHashData())

	// find if ref exists
	var refResult struct {
		ID   int64
		Type string
	}
	cachedRef := collector.GetFromCache(newFile.LookupHash)
	if cachedRef != nil {
		refResult.ID = cachedRef.ID
		refResult.Type = cachedRef.Type
	} else {
		db := datastore.GetStore().GetTransaction(ctx)
		err := db.Model(&reference.Ref{}).Select("id", "type").Where("lookup_hash = ?", newFile.LookupHash).Take(&refResult).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
	}
	if refResult.ID > 0 {
		if !nf.CanUpdate {
			return common.NewError("prohibited_allocation_file_options", "Cannot update data in this allocation.")
		}
		deleteRecord := &reference.Ref{
			ID: refResult.ID,
		}
		collector.DeleteRefRecord(deleteRecord)
	}
	// get parent id
	parent := filepath.Dir(nf.Path)
	// create or get parent directory
	parentRef, err := reference.Mkdir(ctx, nf.AllocationID, parent, ts, collector)
	if err != nil {
		return err
	}
	newFile.ParentID = &parentRef.ID
	collector.CreateRefRecord(newFile)

	return err
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
