package allocation

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type MoveFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
}

func (rf *MoveFileChange) DeleteTempFile() error {
	return nil
}

func (rf *MoveFileChange) ApplyChange(ctx context.Context, change *AllocationChange,
	allocationRoot string, ts common.Timestamp) (*reference.Ref, error) {

	srcRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.SrcPath)
	if err != nil {
		return nil, err
	}

	rootRef, err := reference.GetReferenceForHashCalculationFromPaths(ctx, rf.AllocationID,
		[]string{rf.DestPath, filepath.Dir(rf.SrcPath)})

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

	srcParentPath, srcFileName := filepath.Split(rf.SrcPath)
	srcFields, err := common.GetPathFields(srcParentPath)
	if err != nil {
		return nil, err
	}
	dirRef = rootRef
	for i := 0; i < len(srcFields); i++ {
		found := false
		for _, child := range dirRef.Children {
			if child.Name == srcFields[i] {
				dirRef = child
				found = true
				break
			}
		}
		if !found {
			return nil, common.NewError("invalid_reference_path",
				fmt.Sprintf("path %s does not exist", strings.Join(srcFields[:i], "/")))
		}
	}

	var removed bool
	for i, child := range dirRef.Children {
		if child.Name == srcFileName {
			dirRef.RemoveChild(i)
			removed = true
		}
	}
	if !removed {
		return nil, common.NewError("incomplete_move",
			"move operation rejected as it cannot be completed")
	}
	_, err = rootRef.CalculateHash(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, fileRef := range fileRefs {
		stats.NewFileCreated(ctx, fileRef.ID)
	}
	return rootRef, err
}

func (rf *MoveFileChange) processCopyRefs(
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

func (rf *MoveFileChange) CommitToFileStore(ctx context.Context) error {
	return nil
}
