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
	//IsChunked the request is chunked upload
	IsChunked bool
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

type FileStore interface {
	// WriteFile write chunk file into disk
	WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error)
	DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error

	CreateDir(dirName string) error
	DeleteDir(allocationID, dirPath, connectionID string) error

	GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)

	CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error)

	GetFileBlockForChallenge(allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error)
	DeleteFile(allocationID string, contentHash string) error
	GetTotalDiskSizeUsed() (int64, error)
	GetlDiskSizeUsed(allocationID string) (int64, error)
	GetTempPathSize(allocationID string) (int64, error)
	IterateObjects(allocationID string, handler FileObjectHandler) error
	UploadToCloud(fileHash, filePath string) error
	DownloadFromCloud(fileHash, filePath string) error
	SetupAllocation(allocationID string, skipCreate bool) (*StoreAllocation, error)
}

var fileStore FileStore

func GetFileStore() FileStore {
	return fileStore
}
