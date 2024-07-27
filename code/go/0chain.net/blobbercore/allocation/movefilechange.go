package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

type MoveFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
	Type         string `json:"type"`
}

func (rf *MoveFileChange) DeleteTempFile() error {
	return nil
}

func (rf *MoveFileChange) ApplyChange(cctx context.Context,
	ts common.Timestamp, _ map[string]string, collector reference.QueryCollector) error {
	srcLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.SrcPath)
	destLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.DestPath)

	srcRef, err := reference.GetReferenceByLookupHash(cctx, rf.AllocationID, srcLookUpHash)
	if err != nil {
		return err
	}
	exist, err := reference.IsRefExist(cctx, rf.AllocationID, rf.DestPath)
	if err != nil {
		return err
	}
	if exist {
		return common.NewError("invalid_reference_path", "file already exists")
	}

	if srcRef.Type == reference.DIRECTORY {
		isEmpty, err := reference.IsDirectoryEmpty(cctx, srcRef.ID)
		if err != nil {
			return err
		}
		if !isEmpty {
			return common.NewError("invalid_reference_path", "directory is not empty")
		}
	}
	rf.Type = srcRef.Type

	parentDir, err := reference.Mkdir(cctx, rf.AllocationID, filepath.Dir(rf.DestPath), ts, collector)
	if err != nil {
		return err
	}

	deleteRef := &reference.Ref{
		ID: srcRef.ID,
	}
	collector.DeleteRefRecord(deleteRef)

	srcRef.ID = 0
	srcRef.ParentID = &parentDir.ID
	srcRef.Path = rf.DestPath
	srcRef.ParentPath = filepath.Dir(rf.DestPath)
	srcRef.Name = filepath.Base(rf.DestPath)
	srcRef.LookupHash = destLookUpHash
	srcRef.CreatedAt = ts
	srcRef.UpdatedAt = ts
	srcRef.PathLevel = len(strings.Split(strings.TrimRight(rf.DestPath, "/"), "/"))
	srcRef.FileMetaHash = encryption.Hash(srcRef.GetFileHashData())
	collector.CreateRefRecord(srcRef)

	return nil
}

func (rf *MoveFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *MoveFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *MoveFileChange) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	if rf.Type == reference.DIRECTORY {
		return nil
	}
	srcLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.SrcPath)
	destLookUpHash := reference.GetReferenceLookup(rf.AllocationID, rf.DestPath)
	return filestore.GetFileStore().CopyFile(rf.AllocationID, srcLookUpHash, destLookUpHash)
}

func (rf *MoveFileChange) GetPath() []string {
	return []string{rf.DestPath, rf.SrcPath}
}
