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
	WriteFile(allocID, connID string, fileData *FileInputData, infile multipart.File) (*FileOutputData, error)
	CommitWrite(allocID, connID string, fileData *FileInputData) (bool, error)
	DeleteTempFile(allocID, connID string, fileData *FileInputData) error
	DeleteFile(allocID string, contentHash string) error
	GetFileBlock(allocID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)
	GetFileBlockForChallenge(allocID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error)

	// fPath --> local path of file that is being uploaded
	MinioUpload(contentHash, fPath string) error
	MinioDelete(contentHash string) error
	// fPath --> local path to download file to
	MinioDownload(contentHash, fPath string) error
	GetTotalTempFilesSizeByAllocations() (s uint64)
	GetTempFilesSizeByAllocation(allocID string) uint64
	GetTotalPermFilesSizeByAllocations() uint64
	GetPermFilesSizeByAllocation(allocID string) uint64
	GetTotalFilesSizeByAllocations() uint64
	GetTotalFilesSizeByAllocation(allocID string) uint64

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
