package allocation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/stats"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/util"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

type NewFileChange struct {
	ConnectionID        string               `json:"connection_id" validation:"required"`
	AllocationID        string               `json:"allocation_id"`
	Filename            string               `json:"filename" validation:"required"`
	ThumbnailFilename   string               `json:"thumbnail_filename"`
	Path                string               `json:"filepath" validation:"required"`
	Size                int64                `json:"size"`
	Hash                string               `json:"content_hash,omitempty"`
	ThumbnailSize       int64                `json:"thumbnail_size"`
	ThumbnailHash       string               `json:"thumbnail_content_hash,omitempty"`
	MerkleRoot          string               `json:"merkle_root,omitempty"`
	ActualHash          string               `json:"actual_hash,omitempty" validation:"required"`
	ActualSize          int64                `json:"actual_size,omitempty" validation:"required"`
	ActualThumbnailSize int64                `json:"actual_thumb_size"`
	ActualThumbnailHash string               `json:"actual_thumb_hash"`
	MimeType            string               `json:"mimetype,omitempty"`
	EncryptedKey        string               `json:"encrypted_key,omitempty"`
	CustomMeta          string               `json:"custom_meta,omitempty"`
	Attributes          reference.Attributes `json:"attributes,omitempty"`
}

func (nf *NewFileChange) ProcessChange(ctx context.Context,
	change *AllocationChange, allocationRoot string) (*reference.Ref, error) {

	path, _ := filepath.Split(nf.Path)
	path = filepath.Clean(path)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath(ctx, nf.AllocationID, nf.Path)
	if err != nil {
		return nil, err
	}

	dirRef := rootRef
	treelevel := 0
	for {
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

	var newFile = reference.NewFileRef()
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
	newFile.EncryptedKey = nf.EncryptedKey

	if err = newFile.SetAttributes(&nf.Attributes); err != nil {
		return nil, common.NewErrorf("process_new_file_change",
			"setting file attributes: %v", err)
	}

	dirRef.AddChild(newFile)
	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}
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
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}

func (nf *NewFileChange) DeleteTempFile() error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = nf.Filename
	fileInputData.Path = nf.Path
	fileInputData.Hash = nf.Hash
	err := filestore.GetFileStore().DeleteTempFile(nf.AllocationID, fileInputData, nf.ConnectionID)
	if nf.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = nf.ThumbnailFilename
		fileInputData.Path = nf.Path
		fileInputData.Hash = nf.ThumbnailHash
		err = filestore.GetFileStore().DeleteTempFile(nf.AllocationID, fileInputData, nf.ConnectionID)
	}
	return err
}

func (nfch *NewFileChange) CommitToFileStore(ctx context.Context) error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = nfch.Filename
	fileInputData.Path = nfch.Path
	fileInputData.Hash = nfch.Hash
	_, err := filestore.GetFileStore().CommitWrite(nfch.AllocationID, fileInputData, nfch.ConnectionID)
	if err != nil {
		return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}
	if nfch.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = nfch.ThumbnailFilename
		fileInputData.Path = nfch.Path
		fileInputData.Hash = nfch.ThumbnailHash
		_, err := filestore.GetFileStore().CommitWrite(nfch.AllocationID, fileInputData, nfch.ConnectionID)
		if err != nil {
			return common.NewError("file_store_error", "Error committing thumbnail to file store. "+err.Error())
		}
	}
	return nil
}
