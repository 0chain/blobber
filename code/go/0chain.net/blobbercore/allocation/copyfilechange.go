package allocation

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/common/core/util/wmpt"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

type CopyFileChange struct {
	ConnectionID   string `json:"connection_id"`
	AllocationID   string `json:"allocation_id"`
	SrcPath        string `json:"path"`
	DestPath       string `json:"dest_path"`
	Type           string `json:"type"`
	srcLookupHash  string
	destLookupHash string
	storageVersion int
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return nil
}

func (rf *CopyFileChange) ApplyChange(ctx context.Context, rootRef *reference.Ref, change *AllocationChange,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string) (*reference.Ref, error) {

	totalRefs, err := reference.CountRefs(ctx, rf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	srcRef, err := rootRef.GetSrcPath(rf.SrcPath)
	if err != nil {
		return nil, err
	}

	rootRef.UpdatedAt = ts
	rootRef.HashToBeComputed = true

	dirRef := rootRef
	fields, err := common.GetPathFields(rf.DestPath)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(fields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == fields[i] {
				if child.Type == reference.DIRECTORY {
					child.HashToBeComputed = true
					dirRef = child
					dirRef.UpdatedAt = ts
					found = true
				} else {
					return nil, common.NewError("invalid_path",
						fmt.Sprintf("%s is of file type", child.Path))
				}
			}
		}

		if len(dirRef.Children) >= config.Configuration.MaxObjectsInDir {
			return nil, common.NewErrorf("max_objects_in_dir_reached",
				"maximum objects in directory %s reached: %v", dirRef.Path, config.Configuration.MaxObjectsInDir)
		}

		if !found {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = rf.AllocationID
			newRef.Path = filepath.Join("/", strings.Join(fields[:i+1], "/"))
			fileID, ok := fileIDMeta[newRef.Path]
			if !ok || fileID == "" {
				return nil, common.NewError("invalid_parameter",
					fmt.Sprintf("file path %s has no entry in file ID meta", newRef.Path))
			}
			newRef.FileID = fileID
			newRef.ParentPath = filepath.Join("/", strings.Join(fields[:i], "/"))
			newRef.Name = fields[i]
			newRef.HashToBeComputed = true
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	_, err = rf.processCopyRefs(ctx, srcRef, dirRef, allocationRoot, ts, fileIDMeta)
	if err != nil {
		return nil, err
	}

	return rootRef, err
}

func (rf *CopyFileChange) ApplyChangeV2(ctx context.Context, allocationRoot, clientPubKey string, numFiles *atomic.Int32, ts common.Timestamp, hashSignature map[string]string, trie *wmpt.WeightedMerkleTrie, collector reference.QueryCollector) (int64, error) {
	rf.srcLookupHash = reference.GetReferenceLookup(rf.AllocationID, rf.SrcPath)
	rf.destLookupHash = reference.GetReferenceLookup(rf.AllocationID, rf.DestPath)

	var (
		srcRef *reference.Ref
		err    error
	)

	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		srcRef, err = reference.GetReferenceByLookupHash(ctx, rf.AllocationID, rf.srcLookupHash)
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
			isEmpty, err := reference.IsDirectoryEmpty(ctx, srcRef.Path)
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
		return 0, err
	}

	_, err = reference.Mkdir(ctx, rf.AllocationID, filepath.Dir(rf.DestPath), allocationRoot, ts, numFiles, collector)
	if err != nil {
		return 0, err
	}
	rf.storageVersion = 1
	srcRef.ID = 0
	srcRef.Path = rf.DestPath
	srcRef.LookupHash = rf.destLookupHash
	srcRef.CreatedAt = ts
	srcRef.UpdatedAt = ts
	srcRef.ParentPath = filepath.Dir(rf.DestPath)
	srcRef.Name = filepath.Base(rf.DestPath)
	srcRef.PathLevel = len(strings.Split(strings.TrimRight(rf.DestPath, "/"), "/"))
	srcRef.AllocationRoot = allocationRoot
	if srcRef.Type == reference.FILE {
		fileMetaHashRaw := encryption.RawHash(srcRef.GetFileMetaHashDataV2())
		sig, ok := hashSignature[srcRef.LookupHash]
		if !ok {
			return 0, common.NewError("invalid_parameter", "hash signature not found")
		}
		fileHash := encryption.Hash(srcRef.GetFileHashDataV2())
		verify, err := encryption.Verify(clientPubKey, sig, fileHash)
		if err != nil || !verify {
			return 0, common.NewError("invalid_signature", "Signature is invalid")
		}
		decodedKey, _ := hex.DecodeString(srcRef.LookupHash)
		err = trie.Update(decodedKey, fileMetaHashRaw, uint64(srcRef.NumBlocks))
		if err != nil {
			return 0, err
		}
		srcRef.Hash = sig
		srcRef.FileMetaHash = hex.EncodeToString(fileMetaHashRaw)
	}

	collector.CreateRefRecord(srcRef)
	numFiles.Add(1)
	return srcRef.Size, nil
}

func (rf *CopyFileChange) processCopyRefs(
	ctx context.Context, srcRef, destRef *reference.Ref,
	allocationRoot string, ts common.Timestamp, fileIDMeta map[string]string,
) (
	fileRefs []*reference.Ref, err error,
) {

	newRef := *srcRef
	newRef.ID = 0
	newRef.Path = filepath.Join(destRef.Path, srcRef.Name)
	fileID, ok := fileIDMeta[newRef.Path]
	if !ok || fileID == "" {
		return nil, common.NewError("invalid_parameter",
			fmt.Sprintf("file path %s has no entry in fileID meta", newRef.Path))
	}
	newRef.FileID = fileID
	newRef.ParentPath = destRef.Path
	newRef.CreatedAt = ts
	newRef.UpdatedAt = ts
	newRef.HashToBeComputed = true
	destRef.AddChild(&newRef)
	if newRef.Type == reference.DIRECTORY {
		for _, childRef := range srcRef.Children {
			fRefs, err := rf.processCopyRefs(ctx, childRef, &newRef, allocationRoot, ts, fileIDMeta)
			if err != nil {
				return nil, err
			}
			fileRefs = append(fileRefs, fRefs...)
		}
	} else {
		fileRefs = append(fileRefs, &newRef)
	}

	return
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
	if rf.storageVersion == 0 || rf.Type == reference.DIRECTORY {
		return nil
	}

	return filestore.GetFileStore().CopyFile(rf.AllocationID, rf.srcLookupHash, rf.destLookupHash)
}

func (rf *CopyFileChange) GetPath() []string {
	return []string{rf.DestPath, rf.SrcPath}
}
