package filestore

import (
	"mime/multipart"
)

const CHUNK_SIZE = 64 * 1024

type FileInputData struct {
	Name string
	Path string
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
}
