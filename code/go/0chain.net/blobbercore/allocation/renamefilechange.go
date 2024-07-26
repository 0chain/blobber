package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"

	"go.uber.org/zap"
)

type RenameFileChange struct {
	ConnectionID  string `json:"connection_id"`
	AllocationID  string `json:"allocation_id"`
	Path          string `json:"path"`
	NewName       string `json:"new_name"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	CustomMeta    string `json:"custom_meta"`
	MimeType      string `json:"mimetype"`
	newLookupHash string `json:"-"`
}

func (rf *RenameFileChange) DeleteTempFile() error {
	return nil
}

func (rf *RenameFileChange) applyChange(ctx context.Context,
	ts common.Timestamp, fileIDMeta map[string]string, collector reference.QueryCollector) error {

	if rf.Path == "/" {
		return common.NewError("invalid_operation", "cannot rename root path")
	}

	newPath := filepath.Join(filepath.Dir(rf.Path), rf.NewName)
	isFilePresent, err := reference.IsRefExist(ctx, rf.AllocationID, newPath)
	if err != nil {
		logging.Logger.Info("invalid_reference_path", zap.Error(err))
	}

	if isFilePresent {
		return common.NewError("invalid_reference_path", "file already exists")
	}

	ref, err := reference.GetReference(ctx, rf.AllocationID, rf.Path)
	if err != nil {
		return common.NewError("invalid_reference_path", err.Error())
	}
	deleteRef := &reference.Ref{
		ID: ref.ID,
	}
	collector.DeleteRefRecord(deleteRef)
	ref.Name = rf.NewName
	ref.Path = newPath
	ref.ID = 0
	ref.LookupHash = reference.GetReferenceLookup(rf.AllocationID, newPath)
	ref.UpdatedAt = ts
	ref.FileMetaHash = encryption.Hash(ref.GetFileMetaHashData())
	if rf.CustomMeta != "" {
		ref.CustomMeta = rf.CustomMeta
	}
	if rf.MimeType != "" {
		ref.MimeType = rf.MimeType
	}
	ref.IsPrecommit = true
	collector.CreateRefRecord(ref)
	rf.newLookupHash = ref.LookupHash
	return nil
}

func (rf *RenameFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(rf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (rf *RenameFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), rf)
	return err
}

func (rf *RenameFileChange) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {
	if rf.newLookupHash == "" {
		return common.NewError("invalid_reference_path", "new lookup hash is empty")
	}
	oldFileLookupHash := reference.GetReferenceLookup(rf.AllocationID, rf.Path)
	err := filestore.GetFileStore().CopyFile(rf.AllocationID, oldFileLookupHash, rf.newLookupHash)
	if err != nil {
		logging.Logger.Error("CommitToFileStore: CopyFile", zap.Error(err))
	}
	return err
}

func (rf *RenameFileChange) GetPath() []string {
	if rf.Type == reference.DIRECTORY {
		return []string{rf.Path, rf.Path}
	}
	return []string{rf.Path}
}
