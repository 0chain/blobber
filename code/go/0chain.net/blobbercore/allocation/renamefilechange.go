package allocation

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/common/core/util/wmpt"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

type RenameFileChange struct {
	ConnectionID      string `json:"connection_id"`
	AllocationID      string `json:"allocation_id"`
	Path              string `json:"path"`
	NewName           string `json:"new_name"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	newLookupHash     string
	oldFileLookupHash string
	storageVersion    int
}

func (rf *RenameFileChange) DeleteTempFile() error {
	return nil
}

func (rf *RenameFileChange) applyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, _ map[string]string) (*reference.Ref, error) {

	if rf.Path == "/" {
		return nil, common.NewError("invalid_operation", "cannot rename root path")
	}

	newPath := filepath.Join(filepath.Dir(rf.Path), rf.NewName)
	isFilePresent, err := reference.IsRefExist(ctx, rf.AllocationID, newPath)
	if err != nil {
		logging.Logger.Info("invalid_reference_path", zap.Error(err))
	}

	if isFilePresent {
		return nil, common.NewError("invalid_reference_path", "file already exists")
	}

	affectedRef, err := rootRef.GetSrcPath(rf.Path)
	if err != nil {
		return nil, err
	}
	affectedRef.HashToBeComputed = true
	affectedRef.Name = rf.NewName
	affectedRef.Path = newPath
	affectedRef.UpdatedAt = ts
	if affectedRef.Type == reference.FILE {
		affectedRef.IsPrecommit = true
	} else {
		rf.processChildren(ctx, affectedRef, ts)
	}

	parentPath := filepath.Dir(rf.Path)
	fields, err := common.GetPathFields(parentPath)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true
	dirRef := rootRef

	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				dirRef = child
				dirRef.UpdatedAt = ts
				dirRef.HashToBeComputed = true
				found = true
				break
			}
		}

		if !found {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}

	found := false
	for i, child := range dirRef.Children {
		if child.Path == rf.Path {
			dirRef.RemoveChild(i)
			dirRef.AddChild(affectedRef)
			found = true
			break
		}
	}
	if !found {
		return nil, common.NewError("file_not_found", "File to rename not found in blobber")
	}

	return rootRef, nil
}

func (rf *RenameFileChange) ApplyChangeV2(ctx context.Context, allocationRoot, clientPubKey string, _ *atomic.Int32, ts common.Timestamp, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) (int64, error) {
	collector.LockTransaction()
	defer collector.UnlockTransaction()

	if rf.Path == "/" {
		return 0, common.NewError("invalid_operation", "cannot rename root path")
	}

	newPath := filepath.Join(filepath.Dir(rf.Path), rf.NewName)
	isFilePresent, err := reference.IsRefExist(ctx, rf.AllocationID, newPath)
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Info("invalid_reference_path", zap.Error(err))
		return 0, err
	}

	if isFilePresent {
		return 0, common.NewError("invalid_reference_path", "file already exists")
	}

	oldFileLookupHash := reference.GetReferenceLookup(rf.AllocationID, rf.Path)
	ref, err := reference.GetReferenceByLookupHash(ctx, rf.AllocationID, oldFileLookupHash)
	if err != nil {
		return 0, common.NewError("invalid_reference_path", err.Error())
	}
	if ref.Type == reference.DIRECTORY {
		isEmpty, err := reference.IsDirectoryEmpty(ctx, rf.AllocationID, ref.Path)
		if err != nil {
			return 0, common.NewError("invalid_reference_path", err.Error())
		}
		if !isEmpty {
			return 0, common.NewError("invalid_reference_path", "directory is not empty")
		}
	}
	rf.Type = ref.Type
	deleteRef := &reference.Ref{
		ID:         ref.ID,
		LookupHash: oldFileLookupHash,
		Type:       ref.Type,
	}
	collector.DeleteRefRecord(deleteRef)

	ref.ID = 0
	ref.LookupHash = reference.GetReferenceLookup(rf.AllocationID, newPath)
	collector.CreateRefRecord(ref)
	ref.Name = rf.NewName
	ref.Path = newPath
	ref.CreatedAt = ts
	ref.UpdatedAt = ts
	ref.AllocationRoot = allocationRoot
	if ref.Type == reference.FILE {
		fileMetaHashRaw := encryption.RawHash(ref.GetFileMetaHashDataV2())
		decodedOldKey, _ := hex.DecodeString(oldFileLookupHash)
		err = trie.Update(decodedOldKey, nil, 0)
		if err != nil {
			return 0, err
		}
		decodedNewKey, _ := hex.DecodeString(ref.LookupHash)
		err = trie.Update(decodedNewKey, fileMetaHashRaw, uint64(ref.NumBlocks))
		if err != nil {
			return 0, err
		}
		ref.FileMetaHash = hex.EncodeToString(fileMetaHashRaw)
	}
	rf.newLookupHash = ref.LookupHash
	rf.oldFileLookupHash = oldFileLookupHash
	rf.storageVersion = 1
	return 0, nil
}

func (rf *RenameFileChange) processChildren(ctx context.Context, curRef *reference.Ref, ts common.Timestamp) {
	for _, childRef := range curRef.Children {
		childRef.UpdatedAt = ts
		childRef.HashToBeComputed = true
		newPath := filepath.Join(curRef.Path, childRef.Name)
		childRef.UpdatePath(newPath, curRef.Path)
		if childRef.Type == reference.FILE {
			childRef.IsPrecommit = true
		}
		if childRef.Type == reference.DIRECTORY {
			rf.processChildren(ctx, childRef, ts)
		}
	}
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
	if rf.storageVersion == 0 || rf.Type == reference.DIRECTORY {
		return nil
	}
	err := filestore.GetFileStore().CopyFile(rf.AllocationID, rf.oldFileLookupHash, rf.newLookupHash)
	if err != nil {
		logging.Logger.Error("error_copying_file", zap.Error(err))
	}
	return err
}

func (rf *RenameFileChange) GetPath() []string {
	if rf.Type == reference.DIRECTORY {
		return []string{rf.Path, rf.Path}
	}
	return []string{rf.Path}
}
