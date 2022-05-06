//go:build !integration
// +build !integration

package filestore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/gosdk/core/util"
	"github.com/0chain/gosdk/zboxcore/sdk"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

const (
	KB = 1024
)

var hexCharacters = []byte("abcdef0123456789")

func randString(l int) string {
	var s string
	r := rand.New(rand.NewSource(time.Now().Unix()))
	for i := 0; i < l; i++ {
		c := hexCharacters[r.Intn(len(hexCharacters))]
		s += string(c)
	}
	return s
}
func TestStoreState(t *testing.T) {
	fs := FileStore{
		mAllocs: make(map[string]*allocation),
		rwMU:    &sync.RWMutex{},
		allocMu: &sync.Mutex{},
	}

	allocs := map[string]*allocation{
		randString(64): {
			mu:            &sync.Mutex{},
			allocatedSize: 655360000,
			filesNumber:   1000,
			filesSize:     65536000,
			tmpMU:         &sync.Mutex{},
			tmpFileSize:   6553600,
		},
	}

	for allocID, alloc := range allocs {
		fs.setAllocation(allocID, alloc)
		require.Equal(t, alloc, fs.getAllocation(allocID))

		fs.removeAllocation(allocID)
		require.NotEqual(t, alloc, fs.getAllocation(allocID))

		fs.setAllocation(allocID, alloc)
		size := alloc.filesSize
		n := alloc.filesNumber
		fs.incrDecrAllocFileSizeAndNumber(allocID, -65536, -1)

		expectedSize := size - 65536
		expectedN := n - 1

		require.Equal(t, expectedSize, alloc.filesSize)
		require.Equal(t, expectedN, alloc.filesNumber)

		curAllocatedSize := alloc.allocatedSize
		fs.UpdateAllocationMetaData(map[string]interface{}{"allocation_id": allocID, "allocated_size": int64(25536)})
		require.NotEqual(t, curAllocatedSize, alloc.allocatedSize)

		curAllocatedSize = alloc.allocatedSize
		fs.UpdateAllocationMetaData(map[string]interface{}{"allocation_id": allocID, "allocated_size": int(65536000)})
		require.Equal(t, curAllocatedSize, alloc.allocatedSize)

	}

	var totalDiskUsed uint64
	for _, alloc := range allocs {
		totalDiskUsed += alloc.filesSize + alloc.tmpFileSize
	}

	actualDiskUsed := fs.GetDiskUsedByAllocations()

	require.Equal(t, totalDiskUsed, actualDiskUsed)

}

type initParams struct {
	allocID       string
	allocatedSize uint64
	usedSize      uint64
	totalRefs     uint64
}

func TestStore(t *testing.T) {

	ip := initParams{
		allocID:       randString(64),
		allocatedSize: uint64(655360000),
		usedSize:      6553600,
		totalRefs:     1000}

	mock := datastore.MockTheStore(t)
	setupMockForFileManagerInit(mock, ip)

	fs := FileStore{
		mAllocs: make(map[string]*allocation),
		rwMU:    &sync.RWMutex{},
		allocMu: &sync.Mutex{},
	}

	err := fs.initMap()
	require.Nil(t, err)

	alloc := fs.getAllocation(ip.allocID)
	require.NotNil(t, alloc)

	require.Equal(t, ip.allocatedSize, alloc.allocatedSize)
	require.Equal(t, ip.totalRefs, alloc.filesNumber)
	require.Equal(t, ip.usedSize, alloc.filesSize)

}

func setupMockForFileManagerInit(mock sqlmock.Sqlmock, ip initParams) {
	aa := sqlmock.AnyArg()
	mock.ExpectBegin()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "allocations"`)).
		WillReturnRows(sqlmock.NewRows(
			[]string{
				"id", "blobber_size", "blobber_size_used",
			},
		).AddRow(
			ip.allocID, ip.allocatedSize, ip.usedSize,
		),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "reference_objects" WHERE`)).
		WithArgs(aa, aa, aa).
		WillReturnRows(
			sqlmock.NewRows([]string{"count"}).AddRow(ip.totalRefs),
		)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT sum(size) as file_size FROM "reference_objects" WHERE`)).
		WillReturnRows(
			sqlmock.NewRows([]string{"file_size"}).AddRow(ip.usedSize),
		)

	mock.ExpectClose()

}

func TestValidateDirLevels(t *testing.T) {
	f1 := getDirLevelsForAllocations
	f2 := getDirLevelsForFiles

	defer func() {
		getDirLevelsForAllocations = f1
		getDirLevelsForFiles = f2
	}()

	err := validateDirLevels() // Test Default values
	require.Nil(t, err)

	config.Configuration.AllocDirLevel = []int{-1}

	err = validateDirLevels()
	require.NotNil(t, err)

	config.Configuration.AllocDirLevel = []int{72}
	err = validateDirLevels()
	require.NotNil(t, err)
}

func TestStoreStorageWriteAndCommit(t *testing.T) {

	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	// Temporary fix; waiting for devops team. Actual requirement is false value
	require.True(t, fs.isMountPoint())

	type input struct {
		testName   string
		allocID    string
		connID     string
		fileName   string
		remotePath string
		alloc      *allocation

		differentHash         bool
		shouldCommit          bool
		expectedErrorOnCommit bool
	}

	tests := []input{
		{
			testName: "Should succeed",

			allocID:    randString(64),
			connID:     randString(64),
			fileName:   randString(5),
			remotePath: filepath.Join("/", randString(5)+".txt"),
			alloc: &allocation{
				mu:    &sync.Mutex{},
				tmpMU: &sync.Mutex{},
			},
			shouldCommit:          true,
			expectedErrorOnCommit: false,
		},
		{
			testName:   "Should fail",
			allocID:    randString(64),
			connID:     randString(64),
			fileName:   randString(5),
			remotePath: filepath.Join("/", randString(5)+".txt"),
			alloc: &allocation{
				mu:    &sync.Mutex{},
				tmpMU: &sync.Mutex{},
			},

			differentHash:         true,
			shouldCommit:          true,
			expectedErrorOnCommit: true,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			fs.setAllocation(test.allocID, test.alloc)
			fPath := filepath.Join(fs.mp, randString(10)+".txt")
			contentHash, commitContentHash, err := generateRandomData(fPath)
			require.Nil(t, err)

			fid := &FileInputData{
				Name:      test.fileName,
				Path:      test.remotePath,
				Hash:      contentHash,
				ChunkSize: 64 * KB,
			}

			f, err := os.Open(fPath)
			require.Nil(t, err)
			defer f.Close()

			fod, err := fs.WriteFile(test.allocID, test.connID, fid, f)
			require.Nil(t, err)

			pathHash := encryption.Hash(test.remotePath)
			tempFilePath := fs.getTempPathForFile(test.allocID, test.fileName, pathHash, test.connID)
			tF, err := os.Stat(tempFilePath)
			require.Nil(t, err)

			finfo, err := f.Stat()
			require.Nil(t, err)

			require.Equal(t, finfo.Size(), tF.Size())
			require.Equal(t, fid.Hash, fod.ContentHash)

			if !test.shouldCommit {
				return
			}

			if test.differentHash {
				fid.Hash = randString(64)
			} else {
				fid.Hash = commitContentHash
			}

			success, err := fs.CommitWrite(test.allocID, test.connID, fid)
			if test.expectedErrorOnCommit {
				require.NotNil(t, err)
				require.False(t, success)
			} else {
				require.Nil(t, err)
				require.True(t, success)
			}
		})
	}

}

func TestGetFileBlock(t *testing.T) {
	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	allocID := randString(64)
	alloc := &allocation{
		mu:    &sync.Mutex{},
		tmpMU: &sync.Mutex{},
	}
	fs.setAllocation(allocID, alloc)
	fPath := filepath.Join(fs.mp, randString(10)+".txt")
	_, fileHash, err := generateRandomData(fPath)
	require.Nil(t, err)

	permanentFPath, err := fs.GetPathForFile(allocID, fileHash)
	require.Nil(t, err)

	err = os.MkdirAll(filepath.Dir(permanentFPath), 0777)
	require.Nil(t, err)

	err = os.Rename(fPath, permanentFPath)
	require.Nil(t, err)

	type input struct {
		testName  string
		blockNum  int64
		numBlocks int64

		hash             string
		expectedError    bool
		errorContains    string
		expectedDataSize int
	}

	tests := []input{
		{
			testName:      "start block less than 1",
			blockNum:      0,
			numBlocks:     10,
			hash:          fileHash,
			expectedError: true,
			errorContains: "invalid_block_number",
		},
		{
			testName:      "start block greater than max block num",
			blockNum:      20,
			numBlocks:     10,
			hash:          fileHash,
			expectedError: true,
			errorContains: "invalid_block_number",
		}, {
			testName:      "Non-existing file",
			blockNum:      1,
			numBlocks:     10,
			hash:          randString(64),
			expectedError: true,
			errorContains: "file_exist_error",
		},
		{
			testName:         "successful response",
			blockNum:         1,
			numBlocks:        10,
			expectedDataSize: 10 * 64 * KB,
			hash:             fileHash,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			fid := &FileInputData{
				Hash:      test.hash,
				ChunkSize: 64 * KB,
			}

			data, err := fs.GetFileBlock(allocID, fid, test.blockNum, test.numBlocks)
			if test.expectedError {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), test.errorContains)
				return
			}

			require.Nil(t, err)
			require.Equal(t, len(data), test.expectedDataSize)

		})
	}

}

func TestGetFileBlockForChallenge(t *testing.T) {
	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	orgFilePath := filepath.Join(fs.mp, randString(5)+".txt")
	_, commitContentHash, err := generateRandomData(orgFilePath)
	require.Nil(t, err)

	f, err := os.Open(orgFilePath)
	require.Nil(t, err)

	mr, err := getMerkleRoot(f)
	require.Nil(t, err)
	t.Logf("Merkle root: %s", mr)
	allocID := randString(64)
	fPath, err := fs.GetPathForFile(allocID, commitContentHash)
	require.Nil(t, err)

	err = os.MkdirAll(filepath.Dir(fPath), 0777)
	require.Nil(t, err)

	err = os.Rename(orgFilePath, fPath)
	require.Nil(t, err)

	type input struct {
		testName    string
		blockOffset int

		expectedError bool
		errContains   string
	}

	tests := []input{
		{
			testName:      "Negative block offset",
			blockOffset:   -1,
			expectedError: true,
			errContains:   "invalid_block_number",
		},
		{
			testName:      "Block offset greater than limit",
			blockOffset:   1025,
			expectedError: true,
			errContains:   "invalid_block_number",
		},
		{
			testName:    "Block offset 22",
			blockOffset: 22,
		},
		{
			testName:    "Block offset 23",
			blockOffset: 23,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			fd := &FileInputData{
				Hash:      commitContentHash,
				ChunkSize: 64 * KB,
			}
			rb, fixedMtI, err := fs.GetFileBlockForChallenge(allocID, fd, test.blockOffset)

			if test.expectedError {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), test.errContains)
				return
			}

			require.Nil(t, err)

			require.Equal(t, mr, fixedMtI.GetRoot())

			_ = rb // TODO work. Need to add test if return bytes can satisfy merkle tree with the given merkle  tree
		})
	}
}

func setupStorage(t *testing.T) (*FileStore, func()) {

	wd, err := os.Getwd()
	require.Nil(t, err)

	mountPoint := filepath.Join(wd, randString(20))
	err = os.Mkdir(mountPoint, 0777)
	require.Nil(t, err)

	fs := FileStore{
		mp:      mountPoint,
		mAllocs: make(map[string]*allocation),
		allocMu: &sync.Mutex{},
		rwMU:    &sync.RWMutex{},
	}

	f := func() {
		err := os.RemoveAll(mountPoint)
		require.Nil(t, err)
	}

	return &fs, f
}

func generateRandomData(fPath string) (string, string, error) {
	p := make([]byte, 64*KB*10)
	_, err := rand.Read(p)
	if err != nil {
		return "", "", err
	}
	f, err := os.Create(fPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	cHW := sha256.New()
	mW := io.MultiWriter(f, cHW)

	_, err = mW.Write(p)
	if err != nil {
		return "", "", err
	}

	contentHash := hex.EncodeToString(cHW.Sum(nil))

	hasher := sdk.CreateHasher(64 * KB)
	var count int
	for start := 0; start < len(p); start = start + 64*KB {
		h := sha256.New()
		data := p[start : start+64*KB]
		_, err := h.Write(data)
		if err != nil {
			return "", "", err
		}

		err = hasher.WriteHashToContent(hex.EncodeToString(h.Sum(nil)), count)
		if err != nil {
			return "", "", err
		}
		count++
	}
	commitContentHash, err := hasher.GetContentHash()
	if err != nil {
		return "", "", err
	}

	return contentHash, commitContentHash, nil
}

func getMerkleRoot(r io.Reader) (mr string, err error) {
	fixedMT := util.NewFixedMerkleTree(64 * KB)
	var count int
mainloop:
	for {

		b := make([]byte, 64*KB)
		var n int
		n, err = r.Read(b)
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = nil
				if n == 0 {
					break
				}
				goto final
			}
			return
		}
		if n != 64*KB {
			return "", errors.New("invalid byte length. Must be 64 KB")
		}

		err = fixedMT.Write(b, count)
		if err != nil {
			return
		}

		count++
		continue
	final:
		if n != 64*KB {
			return "", errors.New("invalid byte length. Must be 64 KB")
		}

		err = fixedMT.Write(b, count)
		if err != nil {
			return
		}

		break mainloop
	}

	mr = fixedMT.GetMerkleTree().GetRoot()
	return
}
