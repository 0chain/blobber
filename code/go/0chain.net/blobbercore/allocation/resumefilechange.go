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

// ResumeFileChange file change processor for continuous upload in INIT/APPEND/FINALIZE
type ResumeFileChange struct {
	NewFileChange

	//TrustedConentHasher *gosdk.TrustedConentHasher `json:"trusted_conent_hasher,omitempty"` // streaming merkle hasher to save current state of tree
	IsFinal      bool  `json:"is_final,omitempty"`      // current chunk is last or not
	ChunkIndex   int   `json:"chunk_index,omitempty"`   // the seq of current chunk. all chunks MUST be uploaded one by one because of streaming merkle hash
	UploadOffset int64 `json:"upload_offset,omitempty"` // It is next position that new incoming chunk should be append to
}

// ProcessChange update references, and create a new FileRef
func (nf *ResumeFileChange) ProcessChange(ctx context.Context,
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

// Marshal marshal and change to persistent to postgres
func (nf *ResumeFileChange) Marshal() (string, error) {
	ret, err := json.Marshal(nf)
	if err != nil {
		return "", err
	}
	return string(ret), nil
}

// Unmarshal reload and unmarshal change from allocation_changes.input on postgres
func (nf *ResumeFileChange) Unmarshal(input string) error {
	if err := json.Unmarshal([]byte(input), nf); err != nil {
		return err
	}

	return util.UnmarshalValidation(nf)
}

// DeleteTempFile delete temp files from allocation's temp dir
func (nf *ResumeFileChange) DeleteTempFile() error {
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

// CommitToFileStore move files from temp dir to object dir
func (nf *ResumeFileChange) CommitToFileStore(ctx context.Context) error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = nf.Filename
	fileInputData.Path = nf.Path
	fileInputData.Hash = nf.Hash
	_, err := filestore.GetFileStore().CommitWrite(nf.AllocationID, fileInputData, nf.ConnectionID)
	if err != nil {
		return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}
	if nf.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = nf.ThumbnailFilename
		fileInputData.Path = nf.Path
		fileInputData.Hash = nf.ThumbnailHash
		_, err := filestore.GetFileStore().CommitWrite(nf.AllocationID, fileInputData, nf.ConnectionID)
		if err != nil {
			return common.NewError("file_store_error", "Error committing thumbnail to file store. "+err.Error())
		}
	}
	return nil
}
