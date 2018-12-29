package filestore

import (
	"encoding/json"
	"mime/multipart"
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
	WriteFile(allocationID string, fileData *FileInputData, hdr *multipart.FileHeader) (*FileOutputData, error)
	DeleteFile(allocationID string, fileData *FileInputData) error
	GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64) (json.RawMessage, error)
}
