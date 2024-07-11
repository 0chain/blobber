package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type NewDir struct {
	ConnectionID string `json:"connection_id" validation:"required"`
	Path         string `json:"filepath" validation:"required"`
	AllocationID string `json:"allocation_id"`
}

func (nf *NewDir) ApplyChange(ctx context.Context,
	ts common.Timestamp, fileIDMeta map[string]string, collector reference.QueryCollector) error {

	newRef := reference.NewDirectoryRef()
	newRef.AllocationID = nf.AllocationID
	newRef.Path = nf.Path
	newRef.LookupHash = reference.GetReferenceLookup(nf.AllocationID, newRef.Path)
	newRef.PathLevel = len(strings.Split(strings.TrimRight(newRef.Path, "/"), "/"))
	newRef.ParentPath = filepath.Dir(newRef.Path)
	newRef.Name = filepath.Base(newRef.Path)
	newRef.LookupHash = reference.GetReferenceLookup(nf.AllocationID, newRef.Path)
	newRef.CreatedAt = ts
	newRef.UpdatedAt = ts
	newRef.HashToBeComputed = true
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		//check if ref exists
		exists, err := reference.IsRefExist(ctx, nf.AllocationID, newRef.Path)
		if err != nil {
			return err
		}
		if !exists {
			collector.CreateRefRecord(newRef)
		}
		return nil
	})
	return err
}

func (nd *NewDir) Marshal() (string, error) {
	ret, err := json.Marshal(nd)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nd *NewDir) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nd); err != nil {
		return err
	}

	return util.UnmarshalValidation(nd)
}

func (nf *NewDir) DeleteTempFile() error {
	return nil
}

func (nfch *NewDir) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	return nil
}

func (nfc *NewDir) GetPath() []string {
	return []string{nfc.Path}
}
