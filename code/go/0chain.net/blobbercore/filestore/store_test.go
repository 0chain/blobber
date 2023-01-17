package filestore

import (
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/util"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func init() {
	logging.Logger = zap.NewNop()
}

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
		_ = fs.UpdateAllocationMetaData(map[string]interface{}{"allocation_id": allocID, "allocated_size": int64(25536)})
		require.NotEqual(t, curAllocatedSize, alloc.allocatedSize)

		curAllocatedSize = alloc.allocatedSize
		_ = fs.UpdateAllocationMetaData(map[string]interface{}{"allocation_id": allocID, "allocated_size": int(65536000)})
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
		WithArgs(aa, aa).
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
			size := 640 * KB
			validationRoot, fixedMerkleRoot, err := generateRandomData(fPath, int64(size))
			require.Nil(t, err)

			fid := &FileInputData{
				Name:            test.fileName,
				Path:            test.remotePath,
				ValidationRoot:  validationRoot,
				FixedMerkleRoot: fixedMerkleRoot,
				ChunkSize:       64 * KB,
			}

			f, err := os.Open(fPath)
			require.Nil(t, err)
			defer f.Close()

			_, err = fs.WriteFile(test.allocID, test.connID, fid, f)
			require.Nil(t, err)

			pathHash := encryption.Hash(test.remotePath)
			tempFilePath := fs.getTempPathForFile(test.allocID, test.fileName, pathHash, test.connID)
			tF, err := os.Stat(tempFilePath)
			require.Nil(t, err)

			finfo, err := f.Stat()
			require.Nil(t, err)

			require.Equal(t, finfo.Size(), tF.Size())

			if !test.shouldCommit {
				return
			}

			if test.differentHash {
				fid.ValidationRoot = randString(64)
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
	size := 640 * KB
	validationRoot, _, err := generateRandomDataAndStoreNodes(fPath, int64(size))
	require.Nil(t, err)

	permanentFPath, err := fs.GetPathForFile(allocID, validationRoot)
	require.Nil(t, err)

	err = os.MkdirAll(filepath.Dir(permanentFPath), 0777)
	require.Nil(t, err)

	err = os.Rename(fPath, permanentFPath)
	require.Nil(t, err)

	type input struct {
		testName  string
		blockNum  int64
		numBlocks int64

		validationRoot   string
		expectedError    bool
		errorContains    string
		expectedDataSize int64
	}

	tests := []input{
		{
			testName:       "start block less than 0",
			blockNum:       -1,
			numBlocks:      10,
			validationRoot: validationRoot,
			expectedError:  true,
			errorContains:  "invalid_block_number",
		},
		{
			testName:       "start block greater than max block num",
			blockNum:       20,
			numBlocks:      10,
			validationRoot: validationRoot,
			expectedError:  true,
			errorContains:  "invalid_block_number",
		}, {
			testName:       "Non-existing file",
			blockNum:       1,
			numBlocks:      10,
			validationRoot: randString(64),
			expectedError:  true,
			errorContains:  "no such file or directory",
		},
		{
			testName:         "successful response",
			blockNum:         0,
			numBlocks:        10,
			expectedDataSize: int64(size),
			validationRoot:   validationRoot,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			in := &ReadBlockInput{
				AllocationID:  allocID,
				StartBlockNum: int(test.blockNum),
				NumBlocks:     int(test.numBlocks),
				Hash:          test.validationRoot,
				FileSize:      int64(test.expectedDataSize),
			}

			fileResponse, err := fs.GetFileBlock(in)
			if test.expectedError {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), test.errorContains, "Actual error: ", err.Error())
				return
			}

			require.Nil(t, err)
			require.EqualValues(t, test.expectedDataSize, len(fileResponse.Data))

		})
	}

}

func TestGetMerkleTree(t *testing.T) {
	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	orgFilePath := filepath.Join(fs.mp, randString(5)+".txt")
	size := 640 * KB
	validationRoot, fixedMerkleRoot, err := generateRandomDataAndStoreNodes(orgFilePath, int64(size))
	require.Nil(t, err)

	f, err := os.Open(orgFilePath)
	require.Nil(t, err)

	finfo, _ := f.Stat()
	fmt.Println("Size: ", finfo.Size())
	mr, err := getFixedMerkleRoot(f, int64(size))
	require.Nil(t, err)
	t.Logf("Merkle root: %s", mr)
	allocID := randString(64)
	fPath, err := fs.GetPathForFile(allocID, validationRoot)
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
			cri := &ChallengeReadBlockInput{
				BlockOffset:  test.blockOffset,
				AllocationID: allocID,
				Hash:         validationRoot,
				FileSize:     int64(size),
			}

			challengeProof, err := fs.GetBlocksMerkleTreeForChallenge(cri)

			if test.expectedError {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), test.errContains)
				return
			}

			require.Nil(t, err)

			rootHash, _ := hex.DecodeString(fixedMerkleRoot)
			fmp := &util.FixedMerklePath{
				LeafHash: encryption.RawHash(challengeProof.Data),
				RootHash: rootHash,
				Nodes:    challengeProof.Proof,
				LeafInd:  test.blockOffset,
			}

			require.True(t, fmp.VerifyMerklePath())
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

func generateRandomData(fPath string, size int64) (string, string, error) {
	p := make([]byte, size)
	_, err := rand.Read(p)
	if err != nil {
		return "", "", err
	}
	f, err := os.Create(fPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	cH := GetNewCommitHasher(size)
	_, err = cH.Write(p)
	if err != nil {
		return "", "", err
	}

	err = cH.Finalize()
	if err != nil {
		return "", "", err
	}

	fixedMerkleRoot := cH.fmt.GetMerkleRoot()
	if err != nil {
		return "", "", err
	}

	validationMerkleRoot := cH.vt.GetValidationRoot()
	if err != nil {
		return "", "", err
	}

	_, err = f.Write(p)
	if err != nil {
		return "", "", err
	}

	return hex.EncodeToString(validationMerkleRoot), fixedMerkleRoot, nil
}

func generateRandomDataAndStoreNodes(fPath string, size int64) (string, string, error) {
	p := make([]byte, size)
	_, err := rand.Read(p)
	if err != nil {
		return "", "", err
	}
	f, err := os.Create(fPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	cH := GetNewCommitHasher(size)
	_, err = cH.Write(p)
	if err != nil {
		return "", "", err
	}

	err = cH.Finalize()
	if err != nil {
		return "", "", err
	}

	fixedMerkleRoot, err := cH.fmt.CalculateRootAndStoreNodes(f)
	if err != nil {
		return "", "", err
	}

	validationMerkleRoot, err := cH.vt.CalculateRootAndStoreNodes(f)
	if err != nil {
		return "", "", err
	}

	_, err = f.Write(p)
	if err != nil {
		return "", "", err
	}

	return hex.EncodeToString(validationMerkleRoot), hex.EncodeToString(fixedMerkleRoot), nil
}

func getFixedMerkleRoot(r io.ReadSeeker, dataSize int64) (mr string, err error) {
	_, err = r.Seek(-dataSize, io.SeekEnd)
	if err != nil {
		return
	}

	fixedMT := util.NewFixedMerkleTree()
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
			fmt.Println("n is ", n)
			return "", errors.New("huhu invalid byte length. Must be 64 KB")
		}

		_, err = fixedMT.Write(b)
		if err != nil {
			return
		}

		count++
		continue
	final:
		if n != 64*KB {
			return "", errors.New("hehe invalid byte length. Must be 64 KB")
		}

		_, err = fixedMT.Write(b)
		if err != nil {
			return
		}

		break mainloop
	}

	err = fixedMT.Finalize()
	if err != nil {
		return
	}

	mr = fixedMT.GetMerkleRoot()
	return
}
