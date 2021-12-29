package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

// BaseFileChanger base file change processor
type BaseFileChanger struct {
	// client side: unmarshal them from 'updateMeta'/'uploadMeta'
	ConnectionID string `json:"connection_id" validation:"required"`
	// client side:
	Filename string `json:"filename" validation:"required"`
	// client side:
	Path string `json:"filepath" validation:"required"`
	// client side:
	ActualHash string `json:"actual_hash,omitempty" validation:"required"`
	// client side:
	ActualSize int64 `json:"actual_size,omitempty" validation:"required"`
	// client side:
	ActualThumbnailSize int64 `json:"actual_thumb_size"`
	// client side:
	ActualThumbnailHash string `json:"actual_thumb_hash"`
	// client side:
	MimeType string `json:"mimetype,omitempty"`
	// client side:
	Attributes reference.Attributes `json:"attributes,omitempty"`
	// client side:
	MerkleRoot string `json:"merkle_root,omitempty"`

	// server side: update them by ChangeProcessor
	AllocationID string `json:"allocation_id"`
	// client side:
	Hash string `json:"content_hash,omitempty"`
	Size int64  `json:"size"`
	// server side:
	ThumbnailHash     string `json:"thumbnail_content_hash,omitempty"`
	ThumbnailSize     int64  `json:"thumbnail_size"`
	ThumbnailFilename string `json:"thumbnail_filename"`

	EncryptedKey string `json:"encrypted_key,omitempty"`
	CustomMeta   string `json:"custom_meta,omitempty"`

	ChunkSize int64 `json:"chunk_size,omitempty"` // the size of achunk. 64*1024 is default
	IsFinal   bool  `json:"is_final,omitempty"`   // current chunk is last or not

	ChunkIndex   int    `json:"chunk_index,omitempty"` // the seq of current chunk. all chunks MUST be uploaded one by one because of CompactMerkleTree
	ChunkHash    string `json:"chunk_hash,omitempty"`
	UploadOffset int64  `json:"upload_offset,omitempty"` // It is next position that new incoming chunk should be append to
}

func (nf *BaseFileChanger) DeleteTempFile() error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = nf.Filename
	fileInputData.Path = nf.Path
	fileInputData.Hash = nf.Hash
	alloc, err := VerifyAllocationTransaction(common.GetRootContext(), nf.AllocationID, true)
	if err != nil {
		return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}
	err = filestore.GetFileStore().DeleteTempFile(alloc.AllocationRoot, nf.AllocationID, fileInputData, nf.ConnectionID)
	if nf.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = nf.ThumbnailFilename
		fileInputData.Path = nf.Path
		fileInputData.Hash = nf.ThumbnailHash
		alloc, err := VerifyAllocationTransaction(common.GetRootContext(), nf.AllocationID, true)
		if err != nil {
			return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
		}
		err = filestore.GetFileStore().DeleteTempFile(alloc.AllocationRoot, nf.AllocationID, fileInputData, nf.ConnectionID)
	}
	return err
}

func (nfch *BaseFileChanger) CommitToFileStore(ctx context.Context) error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = nfch.Filename
	fileInputData.Path = nfch.Path
	fileInputData.Hash = nfch.Hash
	alloc, err := VerifyAllocationTransaction(common.GetRootContext(), nfch.AllocationID, true)
	if err != nil {
		return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}
	_, err = filestore.GetFileStore().CommitWrite(alloc.AllocationRoot, nfch.AllocationID, fileInputData, nfch.ConnectionID)
	if err != nil {
		return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}
	if nfch.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = nfch.ThumbnailFilename
		fileInputData.Path = nfch.Path
		fileInputData.Hash = nfch.ThumbnailHash
		alloc, err := VerifyAllocationTransaction(common.GetRootContext(), nfch.AllocationID, true)
		if err != nil {
			return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
		}
		_, err = filestore.GetFileStore().CommitWrite(alloc.AllocationRoot, nfch.AllocationID, fileInputData, nfch.ConnectionID)
		if err != nil {
			return common.NewError("file_store_error", "Error committing thumbnail to file store. "+err.Error())
		}
	}
	return nil
}
