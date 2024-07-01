package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
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
	Hash         string `json:"hash"`
}

func (nf *DeleteFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	err := reference.DeleteObject(ctx, rootRef, nf.AllocationID, filepath.Clean(nf.Path), ts)
	if err != nil {
		return nil, err
	}

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
	return nil
}

func (nf *DeleteFileChange) CommitToFileStore(_ context.Context, _ *sync.Mutex) error {
	return nil
}

func (nf *DeleteFileChange) GetPath() []string {
	return []string{nf.Path}
}
