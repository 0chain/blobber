package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"gorm.io/datatypes"
)

type CopyFileChange struct {
	ConnectionID string `json:"connection_id"`
	AllocationID string `json:"allocation_id"`
	SrcPath      string `json:"path"`
	DestPath     string `json:"dest_path"`
}

func (rf *CopyFileChange) DeleteTempFile() error {
	return OperationNotApplicable
}

func (rf *CopyFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	affectedRef, err := reference.GetObjectTree(ctx, rf.AllocationID, rf.SrcPath)
	if err != nil {
		return nil, err
	}
	affectedRef.HashToBeComputed = true
	if rf.DestPath == "/" {
		destRef, err := reference.GetRefWithSortedChildren(ctx, rf.AllocationID, rf.DestPath)
		if err != nil || destRef.Type != reference.DIRECTORY {
			return nil, common.NewError("invalid_parameters", "Invalid destination path. Should be a valid directory.")
		}
		destRef.HashToBeComputed = true
		rf.processCopyRefs(ctx, affectedRef, destRef, allocationRoot)
		_, err = destRef.CalculateHash(ctx, true)
		return destRef, err
	}

	// it will create new dir if it is not available in db
	destRef, err := reference.Mkdir(ctx, rf.AllocationID, rf.DestPath)
	if err != nil || destRef.Type != reference.DIRECTORY {
		return nil, common.NewError("invalid_parameters", "Invalid destination path. Should be a valid directory.")
	}
	destRef, err = reference.GetRefWithSortedChildren(ctx, rf.AllocationID, rf.DestPath)
	if err != nil || destRef.Type != reference.DIRECTORY {
		return nil, common.NewError("invalid_parameters", "Invalid destination path. Should be a valid directory.")
	}

	path, _ := filepath.Split(rf.DestPath)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath2(ctx, rf.AllocationID, path)
	if err != nil {
		return nil, err
	}
	rootRef.HashToBeComputed = true
	dirRef := rootRef
	treelevel := 0
	for treelevel < len(tSubDirs) {
		found := false
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY && treelevel < len(tSubDirs) {
				if child.Name == tSubDirs[treelevel] {
					dirRef = child
					dirRef.HashToBeComputed = true
					found = true
					break
				}
			}
		}
		if found {
			treelevel++
		} else {
			return nil, common.NewError("invalid_reference_path", "Invalid reference path from the blobber")
		}
	}
	childIndex := -1
	for i, child := range dirRef.Children {
		if child.Path == rf.DestPath && child.Type == reference.DIRECTORY {
			childIndex = i
			break
		}
	}

	if childIndex == -1 {
		return nil, common.NewError("file_not_found", "Destination Object to copy to not found in blobber")
	}

	dirRef.RemoveChild(childIndex)

	rf.processCopyRefs(ctx, affectedRef, destRef, allocationRoot)
	dirRef.AddChild(destRef)
	_, err = rootRef.CalculateHash(ctx, true)
	if err != nil {
		return nil, err
	}

	err = rf.updateWriteMarker(ctx, destRef, affectedRef)

	return rootRef, err
}

func (rf *CopyFileChange) updateWriteMarker(ctx context.Context, destRef, affectedRef *reference.Ref) error {
	ref := destRef
	if affectedRef != nil {
		for _, r := range destRef.Children {
			if affectedRef.Name == r.Name {
				ref = r
			}
		}
	}

	if ref.Type == reference.FILE {
		return stats.NewDirCreated(ctx, ref.ID)
	}

	return rf.updateWriteMarker(ctx, ref, nil)
}

func (rf *CopyFileChange) processCopyRefs(ctx context.Context, affectedRef, destRef *reference.Ref, allocationRoot string) {
	if affectedRef.Type == reference.DIRECTORY {
		newRef := reference.NewDirectoryRef()
		newRef.AllocationID = rf.AllocationID
		newRef.Path = filepath.Join(destRef.Path, affectedRef.Name)
		newRef.ParentPath = destRef.Path
		newRef.Name = affectedRef.Name
		newRef.LookupHash = reference.GetReferenceLookup(newRef.AllocationID, newRef.Path)
		newRef.Attributes = datatypes.JSON(string(affectedRef.Attributes))
		destRef.AddChild(newRef)
		for _, childRef := range affectedRef.Children {
			rf.processCopyRefs(ctx, childRef, newRef, allocationRoot)
		}
	} else {
		newFile := reference.NewFileRef()
		newFile.ActualFileHash = affectedRef.ActualFileHash
		newFile.ActualFileSize = affectedRef.ActualFileSize
		newFile.AllocationID = affectedRef.AllocationID
		newFile.ContentHash = affectedRef.ContentHash
		newFile.CustomMeta = affectedRef.CustomMeta
		newFile.MerkleRoot = affectedRef.MerkleRoot
		newFile.Name = affectedRef.Name
		newFile.ParentPath = destRef.Path
		newFile.Path = filepath.Join(destRef.Path, affectedRef.Name)
		newFile.LookupHash = reference.GetReferenceLookup(newFile.AllocationID, newFile.Path)
		newFile.Size = affectedRef.Size
		newFile.MimeType = affectedRef.MimeType
		newFile.WriteMarker = allocationRoot
		newFile.ThumbnailHash = affectedRef.ThumbnailHash
		newFile.ThumbnailSize = affectedRef.ThumbnailSize
		newFile.ActualThumbnailHash = affectedRef.ActualThumbnailHash
		newFile.ActualThumbnailSize = affectedRef.ActualThumbnailSize
		newFile.EncryptedKey = affectedRef.EncryptedKey
		newFile.Attributes = datatypes.JSON(string(affectedRef.Attributes))
		newFile.ChunkSize = affectedRef.ChunkSize

		destRef.AddChild(newFile)
	}
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
