package allocation

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/common/core/util/wmpt"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	NoNewLock = common.NewError("not_new_lock", "")
)

type DeleteFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	LookupHash   string `json:"lookup_hash"`
}

func (nf *DeleteFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	err := reference.DeleteObject(ctx, rootRef, nf.AllocationID, filepath.Clean(nf.Path), ts)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (nf *DeleteFileChange) ApplyChangeV2(_ context.Context, _, _ string, numFiles *atomic.Int32, _ common.Timestamp, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) (int64, error) {
	var (
		changeSize int64
		refType    string
	)
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		ref, err := reference.GetLimitedRefFieldsByLookupHashWith(ctx, nf.AllocationID, nf.LookupHash, []string{"id", "type", "size"})
		if err != nil {
			logging.Logger.Error("deleted_object_error", zap.Error(err))
			return err
		}
		if ref.Type == reference.DIRECTORY {
			// Recursively soft-delete descendant DIRECTORY refs instead of
			// rejecting a non-empty directory. Root-neutral: the StorageV2
			// merkle trie / allocation_root is built only from type=FILE refs
			// (allocationchange.go ... WHERE type=FILE), so removing directory
			// refs never changes the root and cannot diverge from the client's
			// write marker.
			//
			// FILE descendants are intentionally NOT touched here — the gosdk
			// client cascades file deletes itself (deleteworker.go
			// deleteSubDirectories sends one delete op per descendant file,
			// each carrying its own trie update). If the blobber also removed
			// file descendants its root would diverge from the client's signed
			// marker and the commit would be rejected. So the common
			// streaming-video case (empty /file.mp4/preview dirs that diverged
			// across blobbers) self-heals; a stray divergent FILE on a minority
			// blobber still needs a client-driven delete.
			db := datastore.GetStore().GetTransaction(ctx)
			var descendantDirs []*reference.Ref
			if derr := db.Model(&reference.Ref{}).
				Select("id", "lookup_hash", "type").
				Where("allocation_id=? AND type=? AND path LIKE ?",
					nf.AllocationID, reference.DIRECTORY, nf.Path+"/%").
				Find(&descendantDirs).Error; derr != nil {
				logging.Logger.Error("deleted_object_error", zap.Error(derr))
				return derr
			}
			for _, d := range descendantDirs {
				collector.DeleteRefRecord(&reference.Ref{
					ID:         d.ID,
					LookupHash: d.LookupHash,
					Type:       d.Type,
				})
			}
		}
		deleteRecord := &reference.Ref{
			ID:         ref.ID,
			LookupHash: nf.LookupHash,
			Type:       ref.Type,
		}
		refType = ref.Type
		collector.DeleteRefRecord(deleteRecord)
		changeSize = -ref.Size
		return nil
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	if refType == reference.FILE {
		decodedKey, _ := hex.DecodeString(nf.LookupHash)
		err = trie.Update(decodedKey, nil, 0)
		if err != nil && err != wmpt.ErrNotFound {
			return 0, err
		}
	}
	numFiles.Add(-1)
	return changeSize, nil
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
	return nil
}

func (nf *DeleteFileChange) CommitToFileStore(_ context.Context, _ *sync.Mutex) error {
	return nil
}

func (nf *DeleteFileChange) GetPath() []string {
	return []string{nf.Path}
}
