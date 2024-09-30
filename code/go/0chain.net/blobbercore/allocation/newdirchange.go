package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"gorm.io/gorm"
)

type NewDir struct {
	ConnectionID string `json:"connection_id" validation:"required"`
	Path         string `json:"filepath" validation:"required"`
	AllocationID string `json:"allocation_id"`
	CustomMeta   string `json:"custom_meta,omitempty"`
	MimeType     string `json:"mimetype,omitempty"`
}

func (nf *NewDir) ApplyChange(ctx context.Context,
	ts common.Timestamp, allocationVersion int64, collector reference.QueryCollector) error {
	parentPath := filepath.Dir(nf.Path)
	parentPathLookup := reference.GetReferenceLookup(nf.AllocationID, parentPath)
	parentRef, err := reference.GetFullReferenceByLookupHashWithNewTransaction(parentPathLookup)
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if parentRef == nil || parentRef.ID == 0 {
		_, err = reference.Mkdir(ctx, nf.AllocationID, nf.Path, allocationVersion, ts, collector)
	} else {
		parentIDRef := &parentRef.ID
		newRef := reference.NewDirectoryRef()
		newRef.AllocationID = nf.AllocationID
		newRef.Path = nf.Path
		if newRef.Path != "/" {
			newRef.ParentPath = parentPath
		}
		newRef.Name = filepath.Base(nf.Path)
		newRef.PathLevel = len(strings.Split(strings.TrimRight(nf.Path, "/"), "/"))
		newRef.ParentID = parentIDRef
		newRef.LookupHash = reference.GetReferenceLookup(nf.AllocationID, nf.Path)
		newRef.CreatedAt = ts
		newRef.UpdatedAt = ts
		newRef.FileMetaHash = encryption.FastHash(newRef.GetFileMetaHashData())
		if nf.CustomMeta != "" {
			newRef.CustomMeta = nf.CustomMeta
		}
		if nf.MimeType != "" {
			newRef.MimeType = nf.MimeType
		}
		newRef.AllocationVersion = allocationVersion
		collector.CreateRefRecord(newRef)
	}
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
