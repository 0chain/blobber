package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

// BaseFileChanger base file change processor
type BaseFileChanger struct {
	//client side: unmarshal them from 'updateMeta'/'uploadMeta'
	ConnectionID string `json:"connection_id" validation:"required"`
	//client side:
	Filename string `json:"filename" validation:"required"`
	//client side:
	Path string `json:"filepath" validation:"required"`
	//client side:
	ActualFileHashSignature string `json:"actual_file_hash_signature,omitempty" validation:"required"`
	//client side:
	ActualHash string `json:"actual_hash,omitempty" validation:"required"`
	//client side:
	ActualSize int64 `json:"actual_size,omitempty" validation:"required"`
	//client side:
	ActualThumbnailSize int64 `json:"actual_thumb_size"`
	//client side:
	ActualThumbnailHash string `json:"actual_thumb_hash"`

	//client side:
	MimeType string `json:"mimetype,omitempty"`
	//client side:
	//client side:
	FixedMerkleRoot string `json:"fixed_merkle_root,omitempty"`

	//server side: update them by ChangeProcessor
	AllocationID string `json:"allocation_id"`
	//client side:
	ValidationRootSignature string `json:"validation_root_signature,omitempty"`
	//client side:
	ValidationRoot string `json:"validation_root,omitempty"`
	Size           int64  `json:"size"`
	//server side:
	ThumbnailHash     string `json:"thumbnail_content_hash,omitempty"`
	ThumbnailSize     int64  `json:"thumbnail_size"`
	ThumbnailFilename string `json:"thumbnail_filename"`

	EncryptedKey string `json:"encrypted_key,omitempty"`
	CustomMeta   string `json:"custom_meta,omitempty"`

	ChunkSize int64 `json:"chunk_size,omitempty"` // the size of achunk. 64*1024 is default
	IsFinal   bool  `json:"is_final,omitempty"`   // current chunk is last or not

	ChunkStartIndex int    `json:"chunk_start_index,omitempty"` // start index of chunks.
	ChunkEndIndex   int    `json:"chunk_end_index,omitempty"`   // end index of chunks. all chunks MUST be uploaded one by one because of CompactMerkleTree
	ChunkHash       string `json:"chunk_hash,omitempty"`
	UploadOffset    int64  `json:"upload_offset,omitempty"` // It is next position that new incoming chunk should be append to
}

func (fc *BaseFileChanger) DeleteTempFile() error {
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = fc.Filename
	fileInputData.Path = fc.Path
	fileInputData.ValidationRoot = fc.ValidationRoot
	err := filestore.GetFileStore().DeleteTempFile(fc.AllocationID, fc.ConnectionID, fileInputData)
	if fc.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = fc.ThumbnailFilename
		fileInputData.Path = fc.Path
		fileInputData.ThumbnailHash = fc.ThumbnailHash
		err = filestore.GetFileStore().DeleteTempFile(fc.AllocationID, fc.ConnectionID, fileInputData)
	}
	return err
}

func (fc *BaseFileChanger) CommitToFileStore(ctx context.Context) error {

	if fc.ThumbnailSize > 0 {
		fileInputData := &filestore.FileInputData{}
		fileInputData.Name = fc.ThumbnailFilename
		fileInputData.Path = fc.Path
		fileInputData.ThumbnailHash = fc.ThumbnailHash
		fileInputData.ChunkSize = fc.ChunkSize
		fileInputData.IsThumbnail = true
		_, err := filestore.GetFileStore().CommitWrite(fc.AllocationID, fc.ConnectionID, fileInputData)
		if err != nil {
			return common.NewError("file_store_error", "Error committing thumbnail to file store. "+err.Error())
		}
	}
	fileInputData := &filestore.FileInputData{}
	fileInputData.Name = fc.Filename
	fileInputData.Path = fc.Path
	fileInputData.ValidationRoot = fc.ValidationRoot
	fileInputData.FixedMerkleRoot = fc.FixedMerkleRoot
	fileInputData.ChunkSize = fc.ChunkSize
	fileInputData.Hasher = GetHasher(fc.ConnectionID, encryption.Hash(fc.Path))
	_, err := filestore.GetFileStore().CommitWrite(fc.AllocationID, fc.ConnectionID, fileInputData)
	if err != nil {
		return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}
	return nil
}
