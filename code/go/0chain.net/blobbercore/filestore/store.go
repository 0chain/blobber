package filestore

import (
	"encoding/json"
	"mime/multipart"

	"0chain.net/core/util"
)

const CHUNK_SIZE = 64 * 1024

type FileInputData struct {
	Name    string
	Path    string
	Hash    string
	OnCloud bool
}

type FileOutputData struct {
	Name        string
	Path        string
	MerkleRoot  string
	ContentHash string
	Size        int64
}

type FileObjectHandler func(contentHash string, contentSize int64)

type FileStore interface {
	WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error)
	DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error
	GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)
	CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error)
	//GetMerkleTreeForFile(allocationID string, fileData *FileInputData) (util.MerkleTreeI, error)
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

var fsStore FileStore

func GetFileStore() FileStore {
	return fsStore
}
