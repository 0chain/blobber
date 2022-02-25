package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"

	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/core/util"
)

type MockStore struct {
	d map[string]map[string]bool
}

var mockStore *MockStore

func UseMock(initData map[string]map[string]bool) {
	if mockStore == nil {
		mockStore = &MockStore{d: initData}
	}

	mockStore.d = initData
	fileStore = mockStore
}

// WriteFile write chunk file into disk
func (ms *MockStore) WriteFile(allocationRoot, allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	fileRef := &FileOutputData{}

	fileRef.ChunkUploaded = true

	h := sha256.New()
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

func (ms *MockStore) GetFileBlock(allocationRoot, allocationID string, fileData *FileInputData, blockNum, numBlocks int64) ([]byte, error) {
	return nil, constants.ErrNotImplemented
}

func (ms *MockStore) CommitWrite(allocationRoot, allocationID string, fileData *FileInputData, connectionID string) (bool, error) {
	ms.addFileInDataObj(allocationID, fileData.Hash)
	return true, nil
}

func (ms *MockStore) GetFileBlockForChallenge(allocationRoot, allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error) {
	return nil, nil, constants.ErrNotImplemented
}

func (ms *MockStore) DeleteFile(allocationRoot, allocationID, contentHash string) error {
	if ms.d == nil || ms.d[allocationID] == nil || !ms.d[allocationID][contentHash] {
		return errors.New("file not available related to content")
	}
	delete(ms.d[allocationID], contentHash)

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

func (ms *MockStore) addFileInDataObj(allocationID, contentHash string) {
	if contentHash == "" {
		return
	}
	if ms.d == nil {
		ms.d = make(map[string]map[string]bool, 0)
	}
	dataObj := ms.d[allocationID]
	dataObj[contentHash] = true
}

func (ms *MockStore) SetRootPath(rootPath string) {}
