package allocation

import (
	"context"
	"encoding/json"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"

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
	rootRef, contentHash, err := reference.DeleteObject(ctx, nf.AllocationID, nf.Path)
	if err != nil {
		return nil, err
	}

	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}

	nf.ContentHash = contentHash

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

func (nf *DeleteFileChange) DeleteTempFile() error {
	return OperationNotApplicable
}

func (nf *DeleteFileChange) CommitToFileStore(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	for contenthash := range nf.ContentHash {
		var count int64
		err := db.Table((&reference.Ref{}).TableName()).Where(db.Where(&reference.Ref{ThumbnailHash: contenthash}).Or(&reference.Ref{ContentHash: contenthash})).Where("deleted_at IS null").Where(&reference.Ref{AllocationID: nf.AllocationID}).Count(&count).Error
		if err == nil && count == 0 {
			Logger.Info("Deleting content file", zap.String("content_hash", contenthash))
			if err := filestore.GetFileStore().DeleteFile(nf.AllocationID, contenthash); err != nil {
				Logger.Error("FileStore_DeleteFile", zap.String("allocation_id", nf.AllocationID), zap.Error(err))
			}
		}
	}
	return nil
}
