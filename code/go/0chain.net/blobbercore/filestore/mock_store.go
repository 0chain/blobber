package filestore

import (
	"fmt"
	"mime/multipart"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
)

type MockStore struct {
	FileStore
}

func (fs *MockStore) isMountPoint() bool {
	return true // only for manual testing
}

func (fs *MockStore) Initialize() (err error) {
	fs.mp = config.Configuration.MountPoint
	if !fs.isMountPoint() {
		return fmt.Errorf("%s is not mount point", fs.mp)
	}

	if err = validateDirLevels(); err != nil {
		return
	}

	fs.allocMu = &sync.Mutex{}
	fs.rwMU = &sync.RWMutex{}
	fs.mAllocs = make(map[string]*allocation)

	if err = fs.initMap(); err != nil {
		return
	}

	return nil
}

func (fs *MockStore) WriteFile(allocID, connID string, fileData *FileInputData, infile multipart.File) (*FileOutputData, error) {
	return fs.FileStore.WriteFile(allocID, connID, fileData, infile)
}

func (fs *MockStore) CommitWrite(allocID, connID string, fileData *FileInputData) (bool, error) {
	return fs.FileStore.CommitWrite(allocID, connID, fileData)
}

func (fs *MockStore) DeleteTempFile(allocID, connID string, fileData *FileInputData) error {
	return fs.FileStore.DeleteTempFile(allocID, connID, fileData)
}

func (fs *MockStore) DeleteFile(allocationID, contentHash string, version int) error {
	return fs.FileStore.DeleteFile(allocationID, contentHash, version)
}

func (fs *MockStore) GetFileBlock(rbi *ReadBlockInput) (*FileDownloadResponse, error) {
	return fs.FileStore.GetFileBlock(rbi)
}

func (fs *MockStore) GetBlocksMerkleTreeForChallenge(cri *ChallengeReadBlockInput) (*ChallengeResponse, error) {

	return fs.FileStore.GetBlocksMerkleTreeForChallenge(cri)
}

func (fs *MockStore) GetTotalTempFileSizes() (s uint64) {
	return fs.FileStore.GetTotalTempFileSizes()
}

func (fs *MockStore) GetTempFilesSizeOfAllocation(allocID string) uint64 {
	return fs.FileStore.GetTempFilesSizeOfAllocation(allocID)
}

func (fs *MockStore) GetTotalCommittedFileSize() uint64 {
	return fs.FileStore.GetTotalCommittedFileSize()
}

func (fs *MockStore) GetCommittedFileSizeOfAllocation(allocID string) uint64 {
	return fs.FileStore.GetCommittedFileSizeOfAllocation(allocID)
}

func (fs *MockStore) GetTotalFilesSize() uint64 {
	return fs.FileStore.GetTotalFilesSize()
}

func (fs *MockStore) GetTotalFilesSizeOfAllocation(allocID string) uint64 {
	return fs.FileStore.GetTotalFilesSizeOfAllocation(allocID)
}

func (fs *MockStore) IterateObjects(allocID string, handler FileObjectHandler) error {
	return fs.FileStore.IterateObjects(allocID, handler)
}

func (fs *MockStore) GetCurrentDiskCapacity() uint64 {
	return fs.FileStore.GetCurrentDiskCapacity()
}

func (fs *MockStore) CalculateCurrentDiskCapacity() error {
	return fs.FileStore.CalculateCurrentDiskCapacity()
}

func (fs *MockStore) GetPathForFile(allocID, contentHash string, version int) (string, error) {
	return fs.FileStore.GetPathForFile(allocID, contentHash, version)
}

func (fs *MockStore) UpdateAllocationMetaData(m map[string]interface{}) error {
	return fs.FileStore.UpdateAllocationMetaData(m)
}
