package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type CopyFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return nil
}

func (rf *CopyFileChange) ApplyChange(ctx context.Context, change *AllocationChange,
	allocationRoot string, ts common.Timestamp) (*reference.Ref, error) {

	totalRefs, err := reference.CountRefs(rf.AllocationID)
	if err != nil {
		return nil, err
	}

	if int64(config.Configuration.MaxAllocationDirFiles) <= totalRefs {
		return nil, common.NewErrorf("max_alloc_dir_files_reached",
			"maximum files and directories already reached: %v", err)
	}

	srcRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.SrcPath)
	if err != nil {
		return nil, err
	}

	rootRef, err := reference.GetReferencePath(ctx, rf.AllocationID, rf.DestPath)
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

		if !found {

			if change.Operation == constants.FileOperationCopy {
				return nil, common.NewError("invalid_path",
						fmt.Sprintf("destpath does not exist."))
			}

			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = rf.AllocationID
			newRef.Path = filepath.Join("/", strings.Join(fields[:i+1], "/"))
			newRef.ParentPath = filepath.Join("/", strings.Join(fields[:i], "/"))
			newRef.Name = fields[i]
			newRef.HashToBeComputed = true
			newRef.CreatedAt = ts
			newRef.UpdatedAt = ts
			dirRef.AddChild(newRef)
			dirRef = newRef
		}
	}

	fileRefs := rf.processCopyRefs(ctx, srcRef, dirRef, allocationRoot, ts)

	_, err = rootRef.CalculateHash(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, fileRef := range fileRefs {
		stats.NewFileCreated(ctx, fileRef.ID)
	}
	return rootRef, err
}

func (rf *CopyFileChange) processCopyRefs(
	ctx context.Context, srcRef, destRef *reference.Ref,
	allocationRoot string, ts common.Timestamp) (fileRefs []*reference.Ref) {

	if srcRef.Type == reference.DIRECTORY {
		newRef := reference.NewDirectoryRef()
		newRef.AllocationID = rf.AllocationID
		newRef.Path = filepath.Join(destRef.Path, srcRef.Name)
		newRef.ParentPath = destRef.Path
		newRef.Name = srcRef.Name
		newRef.CreatedAt = ts
		newRef.UpdatedAt = ts
		newRef.HashToBeComputed = true
		destRef.AddChild(newRef)

		for _, childRef := range srcRef.Children {
			fileRefs = append(fileRefs, rf.processCopyRefs(ctx, childRef, newRef, allocationRoot, ts)...)
		}
	} else if srcRef.Type == reference.FILE {
		newFile := reference.NewFileRef()
		newFile.ActualFileHash = srcRef.ActualFileHash
		newFile.ActualFileSize = srcRef.ActualFileSize
		newFile.AllocationID = srcRef.AllocationID
		newFile.ContentHash = srcRef.ContentHash
		newFile.CustomMeta = srcRef.CustomMeta
		newFile.MerkleRoot = srcRef.MerkleRoot
		newFile.Name = srcRef.Name
		newFile.ParentPath = destRef.Path
		newFile.Path = filepath.Join(destRef.Path, srcRef.Name)
		newFile.Size = srcRef.Size
		newFile.MimeType = srcRef.MimeType
		newFile.WriteMarker = allocationRoot
		newFile.ThumbnailHash = srcRef.ThumbnailHash
		newFile.ThumbnailSize = srcRef.ThumbnailSize
		newFile.ActualThumbnailHash = srcRef.ActualThumbnailHash
		newFile.ActualThumbnailSize = srcRef.ActualThumbnailSize
		newFile.EncryptedKey = srcRef.EncryptedKey
		newFile.ChunkSize = srcRef.ChunkSize
		newFile.CreatedAt = ts
		newFile.UpdatedAt = ts
		newFile.HashToBeComputed = true
		destRef.AddChild(newFile)

		fileRefs = append(fileRefs, newFile)
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

func (rf *CopyFileChange) CommitToFileStore(ctx context.Context) error {
	return nil
}
