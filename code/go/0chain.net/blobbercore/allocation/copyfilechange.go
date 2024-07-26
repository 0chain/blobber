package allocation

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type CopyFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
	Type         string `json:"type"`
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return nil
}

func (rf *CopyFileChange) ApplyChange(ctx context.Context,
	ts common.Timestamp, _ map[string]string, collector reference.QueryCollector) error {
	return common.NewError("not_implemented", "not implemented")
}

func (rf *CopyFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *CopyFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *CopyFileChange) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	return nil
}

func (rf *CopyFileChange) GetPath() []string {
	return []string{rf.DestPath, rf.SrcPath}
}
