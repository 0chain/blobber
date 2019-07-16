package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"0chain.net/blobbercore/reference"
	"0chain.net/blobbercore/stats"
)

type NewFileChange struct {
	ConnectionID        string `json:"connection_id"`
	AllocationID        string `json:"allocation_id"`
	Filename            string `json:"filename"`
	ThumbnailFilename   string `json:"thumbnail_filename"`
	Path                string `json:"filepath"`
	Size                int64  `json:"size"`
	Hash                string `json:"content_hash,omitempty"`
	ThumbnailSize       int64  `json:"thumbnail_size"`
	ThumbnailHash       string `json:"thumbnail_content_hash,omitempty"`
	MerkleRoot          string `json:"merkle_root,omitempty"`
	ActualHash          string `json:"actual_hash,omitempty"`
	ActualSize          int64  `json:"actual_size,omitempty"`
	ActualThumbnailSize int64  `json:"actual_thumb_size"`
	ActualThumbnailHash string `json:"actual_thumb_hash"`
	MimeType            string `json:"mimetype,omitempty"`
	CustomMeta          string `json:"custom_meta,omitempty"`
}

func (nf *NewFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	path, _ := filepath.Split(nf.Path)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath(ctx, nf.AllocationID, nf.Path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	treelevel := 0
	for true {
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
			continue
		}
		if len(tSubDirs) > treelevel {
			newRef := reference.NewDirectoryRef()
			newRef.AllocationID = dirRef.AllocationID
			newRef.Path = "/" + strings.Join(tSubDirs[:treelevel+1], "/")
			newRef.ParentPath = "/" + strings.Join(tSubDirs[:treelevel], "/")
			newRef.Name = tSubDirs[treelevel]
			newRef.LookupHash = reference.GetReferenceLookup(dirRef.AllocationID, newRef.Path)
			dirRef.AddChild(newRef)
			dirRef = newRef
			treelevel++
			continue
		} else {
			break
		}
	}
	newFile := reference.NewFileRef()
	newFile.ActualFileHash = nf.ActualHash
	newFile.ActualFileSize = nf.ActualSize
	newFile.AllocationID = dirRef.AllocationID
	newFile.ContentHash = nf.Hash
	newFile.CustomMeta = nf.CustomMeta
	newFile.MerkleRoot = nf.MerkleRoot
	newFile.Name = nf.Filename
	newFile.ParentPath = dirRef.Path
	newFile.Path = nf.Path
	newFile.LookupHash = reference.GetReferenceLookup(dirRef.AllocationID, nf.Path)
	newFile.Size = nf.Size
	newFile.MimeType = nf.MimeType
	newFile.WriteMarker = allocationRoot
	newFile.ThumbnailHash = nf.ThumbnailHash
	newFile.ThumbnailSize = nf.ThumbnailSize
	newFile.ActualThumbnailHash = nf.ActualThumbnailHash
	newFile.ActualThumbnailSize = nf.ActualThumbnailSize

	dirRef.AddChild(newFile)
	rootRef.CalculateHash(ctx, true)
	stats.NewFileCreated(ctx, newFile.ID)
	return rootRef, nil
}

func (nf *NewFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

func (nf *NewFileChange) Unmarshal(input string) error {
	err := json.Unmarshal([]byte(input), nf)
	return err
}
