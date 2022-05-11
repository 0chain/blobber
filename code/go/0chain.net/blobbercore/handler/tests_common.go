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
	"github.com/0chain/gosdk/core/util"
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
		Name:        fileData.Name,
		Path:        fileData.Path,
		MerkleRoot:  "",
		ContentHash: fileData.Hash,
		Size:        n,
	}, nil
}

func (mfs *MockFileStore) CommitWrite(allocID, connID string, fileData *filestore.FileInputData) (bool, error) {
	return true, nil
}

func (mfs *MockFileStore) DeleteTempFile(allocID, connID string, fileData *filestore.FileInputData) error {
	return nil
}

func (mfs *MockFileStore) DeleteFile(allocID string, contentHash string) error {
	return nil
}

func (mfs *MockFileStore) GetFileBlock(allocID string, fileData *filestore.FileInputData, blockNum int64, numBlocks int64) ([]byte, error) {
	return nil, nil
}

func (mfs *MockFileStore) GetMerkleTree(allocID string, fileData *filestore.FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error) {
	return nil, nil, nil
}

func (mfs *MockFileStore) MinioUpload(contentHash, fPath string) error {
	return nil
}

func (mfs *MockFileStore) MinioDelete(contentHash string) error {
	return nil
}

func (mfs *MockFileStore) MinioDownload(contentHash, fPath string) error {
	return nil
}

func (mfs *MockFileStore) GetTotalTempFilesSizeByAllocations() (s uint64) {
	return 0
}

func (mfs *MockFileStore) GetTempFilesSizeByAllocation(allocID string) uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalPermFilesSizeByAllocations() uint64 {
	return 0
}

func (mfs *MockFileStore) GetPermFilesSizeByAllocation(allocID string) uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalFilesSizeByAllocations() uint64 {
	return 0
}

func (mfs *MockFileStore) GetTotalFilesSizeByAllocation(allocID string) uint64 {
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

func (mfs *MockFileStore) UpdateAllocationMetaData(m map[string]interface{}) {

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
