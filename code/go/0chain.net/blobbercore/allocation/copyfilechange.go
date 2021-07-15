package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
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
	destRef, err := reference.GetRefWithSortedChildren(ctx, rf.AllocationID, rf.DestPath)
	if err != nil || destRef.Type != reference.DIRECTORY {
		return nil, common.NewError("invalid_parameters", "Invalid destination path. Should be a valid directory.")
	}

	rf.processCopyRefs(ctx, affectedRef, destRef, allocationRoot)

	if destRef.ParentPath == "" {
		_, err = destRef.CalculateHash(ctx, true)
		return destRef, err
	}

	path, _ := filepath.Split(rf.DestPath)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath(ctx, rf.AllocationID, path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	treelevel := 0
	for treelevel < len(tSubDirs) {
		found := false
		for _, child := range dirRef.Children {
			if child.Type == reference.DIRECTORY && treelevel < len(tSubDirs) {
				if child.Name == tSubDirs[treelevel] {
					dirRef = child
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
	var foundRef *reference.Ref = nil
	for i, child := range dirRef.Children {
		if child.Path == rf.DestPath && child.Type == reference.DIRECTORY {
			foundRef = dirRef.Children[i]
			dirRef.RemoveChild(i)
			dirRef.AddChild(destRef)
			break
		}
	}

	if foundRef == nil {
		return nil, common.NewError("file_not_found", "Destination Object to copy to not found in blobber")
	}

	_, err = rootRef.CalculateHash(ctx, true)

	return rootRef, err
}

func (rf *CopyFileChange) processCopyRefs(ctx context.Context, affectedRef *reference.Ref, destRef *reference.Ref, allocationRoot string) {
	if affectedRef.Type == reference.DIRECTORY {
		newRef := reference.NewDirectoryRef()
		newRef.AllocationID = rf.AllocationID
		newRef.Path = filepath.Join(destRef.Path, affectedRef.Name)
		newRef.ParentPath = destRef.Path
		newRef.Name = affectedRef.Name
		newRef.LookupHash = reference.GetReferenceLookup(newRef.AllocationID, newRef.Path)
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
