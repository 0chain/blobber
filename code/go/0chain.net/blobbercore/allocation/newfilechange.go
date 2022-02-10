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
	"github.com/0chain/gosdk/constants"
)

type NewFileChange struct {
	//client side: unmarshal them from 'updateMeta'/'uploadMeta'
	ConnectionID string `json:"connection_id" validation:"required"`
	//client side:
	Filename string `json:"filename" validation:"required"`
	//client side:
	Path string `json:"filepath" validation:"required"`
	//client side:
	ActualHash string `json:"actual_hash,omitempty" `
	//client side:
	ActualSize int64 `json:"actual_size,omitempty"`
	//client side:
	ActualThumbnailSize int64 `json:"actual_thumb_size"`
	//client side:
	ActualThumbnailHash string `json:"actual_thumb_hash"`
	//client side:
	MimeType string `json:"mimetype,omitempty"`
	//client side:
	Attributes reference.Attributes `json:"attributes,omitempty"`
	//client side:
	MerkleRoot string `json:"merkle_root,omitempty"`

	//server side: update them by ChangeProcessor
	AllocationID string `json:"allocation_id"`
	//client side:
	Hash string `json:"content_hash,omitempty"`
	Size int64  `json:"size"`
	//server side:
	ThumbnailHash     string `json:"thumbnail_content_hash,omitempty"`
	ThumbnailSize     int64  `json:"thumbnail_size"`
	ThumbnailFilename string `json:"thumbnail_filename"`

	EncryptedKey string `json:"encrypted_key,omitempty"`
	CustomMeta   string `json:"custom_meta,omitempty"`

	ChunkSize int64 `json:"chunk_size,omitempty"` // the size of achunk. 64*1024 is default
}

func (nf *NewFileChange) CreateDir(ctx context.Context, allocationID, dirName, allocationRoot string) (*reference.Ref, error) {
	path := filepath.Clean(dirName)
	tSubDirs := reference.GetSubDirsFromPath(path)

	rootRef, err := reference.GetReferencePath(ctx, allocationID, nf.Path)
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

	// adding nil to make childLoaded as true so we can have hash calculated in CalculateHas.
	// without has commit fails
	var newDir = reference.NewDirectoryRef()
	newDir.ActualFileSize = 0
	newDir.AllocationID = dirRef.AllocationID
	newDir.MerkleRoot = nf.MerkleRoot
	newDir.Path = dirName
	newDir.Size = 0
	newDir.NumBlocks = 0
	newDir.ParentPath = dirRef.Path
	newDir.WriteMarker = allocationRoot
	dirRef.AddChild(newDir)

	if _, err := rootRef.CalculateHash(ctx, true); err != nil {
		return nil, err
	}

	if err := stats.NewDirCreated(ctx, dirRef.ID); err != nil {
		return nil, err
	}

	return rootRef, nil
}

func (nf *NewFileChange) ProcessChange(ctx context.Context, change *AllocationChange, allocationRoot string) (*reference.Ref, error) {
	if change.Operation == constants.FileOperationCreateDir {
		err := nf.Unmarshal(change.Input)
		if err != nil {
			return nil, err
		}

		return nf.CreateDir(ctx, nf.AllocationID, nf.Path, allocationRoot)
	}

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
	newFile.ChunkSize = nf.ChunkSize

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
