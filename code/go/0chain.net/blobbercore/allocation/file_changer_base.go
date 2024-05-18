package allocation

import (
	"context"
	"net/http"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
)

// swagger:model BaseFileChanger 
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

	EncryptedKey      string `json:"encrypted_key,omitempty"`
	EncryptedKeyPoint string `json:"encrypted_key_point,omitempty"`
	CustomMeta        string `json:"custom_meta,omitempty"`

	ChunkSize int64 `json:"chunk_size,omitempty"` // the size of achunk. 64*1024 is default
	IsFinal   bool  `json:"is_final,omitempty"`   // current chunk is last or not

	ChunkStartIndex int    `json:"chunk_start_index,omitempty"` // start index of chunks.
	ChunkEndIndex   int    `json:"chunk_end_index,omitempty"`   // end index of chunks. all chunks MUST be uploaded one by one because of CompactMerkleTree
	ChunkHash       string `json:"chunk_hash,omitempty"`
	UploadOffset    int64  `json:"upload_offset,omitempty"` // It is next position that new incoming chunk should be append to
	PathHash        string `json:"-"`                       // hash of path
}

// swagger:model UploadResult
type UploadResult struct {
	Filename        string `json:"filename"`
	Size            int64  `json:"size"`
	Hash            string `json:"hash"`
	ValidationRoot  string `json:"validation_root"`
	FixedMerkleRoot string `json:"fixed_merkle_root"`

	// UploadLength indicates the size of the entire upload in bytes. The value MUST be a non-negative integer.
	UploadLength int64 `json:"upload_length"`
	// Upload-Offset indicates a byte offset within a resource. The value MUST be a non-negative integer.
	UploadOffset int64 `json:"upload_offset"`
	UpdateChange bool  `json:"-"`
}

type FileCommand interface {

	// GetExistingFileRef get file ref if it exists
	GetExistingFileRef() *reference.Ref

	GetPath() string

	// IsValidated validate request, and try build ChangeProcesser instance
	IsValidated(ctx context.Context, req *http.Request, allocationObj *Allocation, clientID string) error

	// ProcessContent flush file to FileStorage
	ProcessContent(allocationObj *Allocation) (UploadResult, error)

	// ProcessThumbnail flush thumbnail file to FileStorage if it has.
	ProcessThumbnail(allocationObj *Allocation) error

	// UpdateChange update AllocationChangeProcessor. It will be president in db for committing transcation
	UpdateChange(ctx context.Context, connectionObj *AllocationChangeCollector) error

	//NumBlocks return number of blocks uploaded by the client
	GetNumBlocks() int64
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

func (fc *BaseFileChanger) CommitToFileStore(ctx context.Context, mut *sync.Mutex) error {

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
	fileInputData.Size = fc.Size
	fileInputData.Hasher = GetHasher(fc.ConnectionID, encryption.Hash(fc.Path))
	if fileInputData.Hasher == nil {
		return common.NewError("invalid_parameters", "Invalid parameters. Error getting hasher for commit.")
	}
	_, err := filestore.GetFileStore().CommitWrite(fc.AllocationID, fc.ConnectionID, fileInputData)
	if err != nil {
		return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
	}
	return nil
}
