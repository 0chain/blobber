package allocation

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

type CopyFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
	Type         string `json:"type"`
	CustomMeta   string `json:"custom_meta"`
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return nil
}

func (rf *CopyFileChange) ApplyChange(ctx context.Context,
	ts common.Timestamp, allocationVersion int64, collector reference.QueryCollector) error {
	srcLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.SrcPath)
	destLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.DestPath)

	var (
		srcRef *reference.Ref
		err    error
	)

	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		srcRef, err = reference.GetReferenceByLookupHash(ctx, rf.AllocationID, srcLookUpHash)
		if err != nil {
			return err
		}
		exist, err := reference.IsRefExist(ctx, rf.AllocationID, rf.DestPath)
		if err != nil {
			return err
		}
		if exist {
			return common.NewError("invalid_reference_path", "file already exists")
		}

		rf.Type = srcRef.Type
		if srcRef.Type == reference.DIRECTORY {
			isEmpty, err := reference.IsDirectoryEmpty(ctx, srcRef.ID)
			if err != nil {
				return err
			}
			if !isEmpty {
				return common.NewError("invalid_reference_path", "directory is not empty")
			}
		}
		return nil
	}, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return err
	}

	parentDir, err := reference.Mkdir(ctx, rf.AllocationID, filepath.Dir(rf.DestPath), allocationVersion, ts, collector)
	if err != nil {
		return err
	}

	srcRef.ID = 0
	srcRef.ParentID = &parentDir.ID
	srcRef.Path = rf.DestPath
	srcRef.LookupHash = destLookUpHash
	srcRef.CreatedAt = ts
	srcRef.UpdatedAt = ts
	srcRef.ParentPath = filepath.Dir(rf.DestPath)
	srcRef.Name = filepath.Base(rf.DestPath)
	srcRef.PathLevel = len(strings.Split(strings.TrimRight(rf.DestPath, "/"), "/"))
	srcRef.FileMetaHash = encryption.Hash(srcRef.GetFileHashData())
	if rf.CustomMeta != "" {
		srcRef.CustomMeta = rf.CustomMeta
	}
	srcRef.AllocationVersion = allocationVersion
	collector.CreateRefRecord(srcRef)

	return nil
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
	if rf.Type == reference.DIRECTORY {
		return nil
	}
	srcLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.SrcPath)
	destLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.DestPath)
	return filestore.GetFileStore().CopyFile(rf.AllocationID, srcLookUpHash, destLookUpHash)
}

func (rf *CopyFileChange) GetPath() []string {
	return []string{rf.DestPath, rf.SrcPath}
}
