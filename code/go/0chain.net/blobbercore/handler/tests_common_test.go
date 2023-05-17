//go:build !integration_tests
// +build !integration_tests

package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/gosdk/core/zcncrypto"
	"github.com/0chain/gosdk/zcncore"
)

func setup(t *testing.T) {
	// setup wallet
	w, err := zcncrypto.NewSignatureScheme("bls0chain").GenerateKeys()
	if err != nil {
		t.Fatal(err)
	}
	wBlob, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	if err := zcncore.SetWalletInfo(string(wBlob), true); err != nil {
		t.Fatal(err)
	}

	// setup servers
	sharderServ := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
			},
		),
	)
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				n := zcncore.Network{Miners: []string{"miner 1"}, Sharders: []string{sharderServ.URL}}
				blob, err := json.Marshal(n)
				if err != nil {
					t.Fatal(err)
				}

				if _, err := w.Write(blob); err != nil {
					t.Fatal(err)
				}
			},
		),
	)

	if err := zcncore.InitZCNSDK(server.URL, "ed25519"); err != nil {
		t.Fatal(err)
	}
}

type MockFileStore struct {
	mp string
}

func (mfs *MockFileStore) Initialize() error {
	return nil
}

func (mfs *MockFileStore) WriteFile(allocID, connID string,
	fileData *filestore.FileInputData, infile multipart.File) (*filestore.FileOutputData, error) {

	b := bytes.NewBuffer(make([]byte, 0))
	n, _ := io.Copy(b, infile)
	return &filestore.FileOutputData{
		Name:            fileData.Name,
		Path:            fileData.Path,
		FixedMerkleRoot: "",
		ValidationRoot:  fileData.ValidationRoot,
		Size:            n,
	}, nil
}

func (mfs *MockFileStore) CommitWrite(allocID, connID string, fileData *filestore.FileInputData) (bool, error) {
	return true, nil
}

func (mfs *MockFileStore) MoveToFilestore(allocID, hash string) error {
	return nil
}

func (mfs *MockFileStore) DeleteFromFilestore(allocID, hash string) error {
	return nil
}

func (mfs *MockFileStore) DeleteTempFile(allocID, connID string, fileData *filestore.FileInputData) error {
	return nil
}

func (mfs *MockFileStore) DeleteFile(allocID, contentHash string) error {
	return nil
}

func (mfs *MockFileStore) DeletePreCommitDir(allocID string) error {
	return nil
}

func (mfs *MockFileStore) GetFileBlock(in *filestore.ReadBlockInput) (*filestore.FileDownloadResponse, error) {
	return &filestore.FileDownloadResponse{
		Data: mockFileBlock,
	}, nil
}

func (mfs *MockFileStore) GetBlocksMerkleTreeForChallenge(cri *filestore.ChallengeReadBlockInput,
) (*filestore.ChallengeResponse, error) {
	return nil, nil
}

func (mfs *MockFileStore) GetTotalTempFileSizes() (s uint64) {
	return 0
}
func (mfs *MockFileStore) GetFilePathSize(allocID, contentHash, thumbHash string) (int64, int64, error) {
	return 0, 0, nil
}

func (mfs *MockFileStore) GetTempFilesSizeOfAllocation(allocID string) uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalCommittedFileSize() uint64 {
	return 0
}

func (mfs *MockFileStore) GetCommittedFileSizeOfAllocation(allocID string) uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalFilesSize() uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalFilesSizeOfAllocation(allocID string) uint64 {
	return 0
}

func (mfs *MockFileStore) IterateObjects(allocationID string, handler filestore.FileObjectHandler) error {
	return nil
}

func (mfs *MockFileStore) GetCurrentDiskCapacity() uint64 {
	return 0
}

func (mfs *MockFileStore) CalculateCurrentDiskCapacity() error {
	return nil
}

// GetPathForFile is based on default directory levels. If directory levels are changed
// getDirLevelsXXX function needs to be changed accordingly
func (mfs *MockFileStore) GetPathForFile(allocID, contentHash string) (string, error) {
	if len(allocID) != 64 || len(contentHash) != 64 {
		return "", errors.New("length of allocationID/contentHash must be 64")
	}

	return filepath.Join(mfs.getAllocDir(allocID), getPath(contentHash, getDirLevelsForFiles())), nil
}

func (mfs *MockFileStore) UpdateAllocationMetaData(m map[string]interface{}) error {
	return nil
}

func (mfs *MockFileStore) getAllocDir(allocID string) string {
	return filepath.Join(mfs.mp, getPath(allocID, getDirLevelsForAllocations()))
}

// getPath returns "/" separated strings with the given levels.
func getPath(hash string, levels []int) string {
	var count int
	var pStr []string
	for _, i := range levels {
		pStr = append(pStr, hash[count:count+i])
		count += i
	}
	pStr = append(pStr, hash[count:])
	return strings.Join(pStr, "/")
}

var getDirLevelsForAllocations = func() []int {
	return []int{2, 1} // default
}

var getDirLevelsForFiles = func() []int {
	return []int{2, 2, 1} // default
}
