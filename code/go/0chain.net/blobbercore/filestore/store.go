package filestore

import (
	"encoding/json"
	"mime/multipart"

	"github.com/0chain/gosdk/core/util"
)

const CHUNK_SIZE = 64 * 1024

type FileInputData struct {
	Name    string
	Path    string
	Hash    string
	OnCloud bool

	// ChunkSize chunk size
	ChunkSize int64
	//UploadLength indicates the size of the entire upload in bytes. The value MUST be a non-negative integer.
	UploadLength int64
	//Upload-Offset indicates a byte offset within a resource. The value MUST be a non-negative integer.
	UploadOffset int64
	//IsFinal  the request is final chunk
	IsFinal bool
}

type FileOutputData struct {
	Name        string
	Path        string
	MerkleRoot  string
	ContentHash string
	// Size wirtten size/chunk size
	Size int64
	// ChunkUploaded the chunk is uploaded or not.
	ChunkUploaded bool
}

type FileObjectHandler func(contentHash string, contentSize int64)

type FileStorer interface {
	// WriteFile write chunk file into disk
	Initialize() error
	WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error)
	CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error)
	DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error
	DeleteFile(allocationID string, contentHash string) error
	GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)
	GetFileBlockForChallenge(allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error)

	MinioUpload(string, string) error
	MinioDelete(string) error

	GetTotalTempFilesSizeByAllocations() (s uint64)
	GetTempFilesSizeByAllocation(allocID string) uint64
	GetTotalPermFilesSizeByAllocations() uint64
	GetPermFilesSizeByAllocation(allocID string) uint64
	GetTotalFilesSizeByAllocations() uint64
	GetTotalFilesSizeByAllocation(allocID string)

	IterateObjects(allocationID string, handler FileObjectHandler) error
	// SetupAllocation(allocationID string, skipCreate bool) (*StoreAllocation, error)
	GetCurrentDiskCapacity() uint64
	CalculateCurrentDiskCapacity() error
	// GetPathForFile given allocation id and content hash of a file, its path is calculated.
	// Will return error if allocation id or content hash are not of length 64
	GetPathForFile(allocID, contentHash string) (string, error)
	UpdateAllocationMetaData(m map[string]interface{})
}

var fileStore FileStorer

func SetFileStore(fs FileStorer) {
	fileStore = fs
}
func GetFileStore() FileStorer {
	return fileStore
}
