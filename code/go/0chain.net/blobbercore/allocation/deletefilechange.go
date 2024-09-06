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

func (nf *DeleteFileChange) ApplyChangeV2(_ context.Context, _, _ string, numFiles *atomic.Int32, _ common.Timestamp, _ map[string]string, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) (int64, error) {
	var changeSize int64
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		ref, err := reference.GetLimitedRefFieldsByLookupHashWith(ctx, nf.AllocationID, nf.LookupHash, []string{"id", "type", "size"})
		if err != nil {
			logging.Logger.Error("deleted_object_error", zap.Error(err))
			return err
		}
		if ref.Type == reference.DIRECTORY {
			isEmpty, err := reference.IsDirectoryEmpty(ctx, nf.Path)
			if err != nil {
				logging.Logger.Error("deleted_object_error", zap.Error(err))
				return err
			}
			if !isEmpty {
				return common.NewError("invalid_reference_path", "directory is not empty")
			}
		}
		deleteRecord := &reference.Ref{
			ID:         ref.ID,
			LookupHash: nf.LookupHash,
			Type:       ref.Type,
		}
		collector.DeleteRefRecord(deleteRecord)
		changeSize = ref.Size
		return nil
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return 0, err
	}
	decodedKey, _ := hex.DecodeString(nf.LookupHash)
	err = trie.Update(decodedKey, nil, 0)
	if err != nil {
		return 0, err
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
