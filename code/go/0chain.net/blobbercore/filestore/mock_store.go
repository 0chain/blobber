package filestore

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/core/util"
)

type MockStore struct {
}

var mockStore *MockStore

func UseMock() {
	if mockStore == nil {
		mockStore = &MockStore{}

	}

	fileStore = mockStore
}

// WriteFile write chunk file into disk
func (ms *MockStore) WriteFile(allocationRoot, allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	fileRef := &FileOutputData{}

	fileRef.ChunkUploaded = true

	h := sha1.New()
	reader := io.TeeReader(infile, h)
	fileSize := int64(0)
	for {

		written, err := io.CopyN(io.Discard, reader, fileData.ChunkSize)

		fileSize += written

		if err != nil {
			break
		}
	}

	fileRef.Size = fileSize
	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))

	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path

	return fileRef, nil
}
func (ms *MockStore) DeleteTempFile(allocationRoot, allocationID string, fileData *FileInputData, connectionID string) error {
	return nil
}

func (ms *MockStore) CreateDir(allocationRoot, allocationID, dirName string) error {
	return nil
}
func (ms *MockStore) DeleteDir(allocationRoot, allocationID, dirPath, connectionID string) error {
	return nil
}

func (ms *MockStore) GetFileBlock(allocationRoot, allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error) {
	return nil, constants.ErrNotImplemented
}

func (ms *MockStore) CommitWrite(allocationRoot, allocationID string, fileData *FileInputData, connectionID string) (bool, error) {
	return true, nil
}

func (ms *MockStore) GetFileBlockForChallenge(allocationRoot, allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error) {
	return nil, nil, constants.ErrNotImplemented
}
func (ms *MockStore) DeleteFile(allocationRoot, allocationID string, contentHash string) error {
	return nil
}
func (ms *MockStore) GetTotalDiskSizeUsed() (int64, error) {
	return 0, constants.ErrNotImplemented
}
func (ms *MockStore) GetlDiskSizeUsed(allocationRoot, allocationID string) (int64, error) {
	return 0, constants.ErrNotImplemented
}
func (ms *MockStore) GetTempPathSize(allocationRoot, allocationID string) (int64, error) {
	return 0, constants.ErrNotImplemented
}
func (ms *MockStore) IterateObjects(allocationRoot, allocationID string, handler FileObjectHandler) error {
	return nil
}
func (ms *MockStore) UploadToCloud(fileHash, filePath string) error {
	return nil
}
func (ms *MockStore) DownloadFromCloud(fileHash, filePath string) error {
	return nil
}
func (ms *MockStore) SetupAllocation(allocationRoot, allocationID string, skipCreate bool) (*StoreAllocation, error) {
	return nil, constants.ErrNotImplemented
}
