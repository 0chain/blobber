package filestore

import (
	"encoding/json"
	"mime/multipart"

	"0chain.net/util"
)

const CHUNK_SIZE = 64 * 1024

type FileInputData struct {
	Name string
	Path string
	Hash string
}

type FileOutputData struct {
	Name        string
	Path        string
	MerkleRoot  string
	ContentHash string
	Size        int64
}

type FileStore interface {
	WriteFile(allocationID string, fileData *FileInputData, hdr *multipart.FileHeader, connectionID string) (*FileOutputData, error)
	DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error
	GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64) (json.RawMessage, error)
	CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error)
	GetMerkleTreeForFile(allocationID string, fileData *FileInputData) (util.MerkleTreeI, error)
	DeleteFile(allocationID string, contentHash string) error
}
