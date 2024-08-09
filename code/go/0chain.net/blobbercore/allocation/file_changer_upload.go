package allocation

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

// swagger:model UploadFileChanger
// UploadFileChanger file change processor for continuous upload in INIT/APPEND/FINALIZE
type UploadFileChanger struct {
	BaseFileChanger
}

// ApplyChange update references, and create a new FileRef
func (nf *UploadFileChanger) applyChange(ctx context.Context,
	ts common.Timestamp, allocationVersion int64, collector reference.QueryCollector) error {

	if nf.AllocationID == "" {
		return common.NewError("invalid_parameter", "allocation_id is required")
	}
	now := time.Now()
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
		LookupHash:              nf.LookupHash,
		DataHash:                nf.DataHash,
		DataHashSignature:       nf.DataHashSignature,
		PathLevel:               len(strings.Split(strings.TrimRight(nf.Path, "/"), "/")),
		NumBlocks:               int64(math.Ceil(float64(nf.Size*1.0) / float64(nf.ChunkSize))),
		FilestoreVersion:        filestore.VERSION,
		AllocationVersion:       allocationVersion,
		NumUpdates:              1,
	}
	newFile.FileMetaHash = encryption.FastHash(newFile.GetFileMetaHashData())
	elapsedNewFile := time.Since(now)
	// find if ref exists
	var refResult struct {
		ID         int64
		Type       string
		NumUpdates int64 `gorm:"column:num_of_updates" json:"num_of_updates"`
	}

	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		tx := datastore.GetStore().GetTransaction(ctx)
		return tx.Model(&reference.Ref{}).Select("id", "type", "num_of_updates").Where("lookup_hash = ?", newFile.LookupHash).Take(&refResult).Error
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	if refResult.ID > 0 {
		if !nf.CanUpdate {
			return common.NewError("prohibited_allocation_file_options", "Cannot update data in this allocation.")
		}
		if refResult.Type != reference.FILE {
			return common.NewError("invalid_reference_path", "Directory already exists with the same path")
		}
		deleteRecord := &reference.Ref{
			ID:         refResult.ID,
			LookupHash: newFile.LookupHash,
			Type:       refResult.Type,
		}
		collector.DeleteRefRecord(deleteRecord)
		newFile.NumUpdates = refResult.NumUpdates + 1
	}
	elapsedNewFileRecord := time.Since(now) - elapsedNewFile
	// get parent id
	parent := filepath.Dir(nf.Path)
	// create or get parent directory
	parentRef, err := reference.Mkdir(ctx, nf.AllocationID, parent, allocationVersion, ts, collector)
	if err != nil {
		return err
	}
	elapsedMkdir := time.Since(now) - elapsedNewFileRecord - elapsedNewFile
	newFile.ParentID = &parentRef.ID
	collector.CreateRefRecord(newFile)
	elapsedCreateRefRecord := time.Since(now) - elapsedMkdir - elapsedNewFileRecord - elapsedNewFile
	logging.Logger.Info("UploadFileChanger", zap.Duration("elapsedNewFile", elapsedNewFile), zap.Duration("elapsedNewFileRecord", elapsedNewFileRecord), zap.Duration("elapsedMkdir", elapsedMkdir), zap.Duration("elapsedCreateRefRecord", elapsedCreateRefRecord), zap.Duration("elapsedTotal", time.Since(now)))
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
