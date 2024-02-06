package filestore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/seqpriorityqueue"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/util"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
)

func init() {
	logging.Logger = zap.NewNop()
}

var hexCharacters = []byte("abcdef0123456789")

func randString(n int) string {
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(hexCharacters))))
		if err != nil {
			return ""
		}
		ret[i] = hexCharacters[num.Int64()]
	}
	return string(ret)
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

	mock.ExpectCommit()

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
			pathHash := encryption.Hash(test.remotePath)
			hasher := GetNewCommitHasher(int64(size))
			fid := &FileInputData{
				Name:            test.fileName,
				Path:            test.remotePath,
				ValidationRoot:  validationRoot,
				FixedMerkleRoot: fixedMerkleRoot,
				ChunkSize:       64 * KB,
				FilePathHash:    pathHash,
				Hasher:          hasher,
				Size:            int64(size),
			}

			f, err := os.Open(fPath)
			require.Nil(t, err)
			defer f.Close()
			_, err = fs.WriteFile(test.allocID, test.connID, fid, f)
			require.Nil(t, err)

			tempFilePath := fs.getTempPathForFile(test.allocID, test.fileName, pathHash, test.connID)
			tF, err := os.Stat(tempFilePath)
			require.Nil(t, err)
			seqPQ := seqpriorityqueue.NewSeqPriorityQueue()
			hasher.WG.Add(1)
			go hasher.Start(context.TODO(), test.connID, test.allocID, test.fileName, pathHash, seqPQ)
			seqPQ.Done(seqpriorityqueue.UploadData{
				Offset:    0,
				DataBytes: tF.Size(),
			})
			finfo, err := f.Stat()
			require.Nil(t, err)
			nodeSize := getNodesSize(int64(finfo.Size()), util.MaxMerkleLeavesSize)
			require.Equal(t, finfo.Size(), tF.Size()-nodeSize-FMTSize)

			if !test.shouldCommit {
				return
			}

			if test.differentHash {
				fid.ValidationRoot = randString(64)
			}
			success, err := fs.CommitWrite(test.allocID, test.connID, fid)
			errr := fs.MoveToFilestore(test.allocID, validationRoot)
			require.Nil(t, errr)
			if test.expectedErrorOnCommit {
				if err == nil {
					success, err = fs.CommitWrite(test.allocID, test.connID, fid)
				}
				require.NotNil(t, err)
				require.False(t, success)
			} else {
				require.Nil(t, err)
				require.True(t, success)
				preCommitPath := fs.getPreCommitPathForFile(test.allocID, fid.ValidationRoot)
				_, err := os.Open(preCommitPath)
				require.NotNil(t, err)
				finalPath, err := fs.GetPathForFile(test.allocID, fid.ValidationRoot)
				require.Nil(t, err)
				_, err = os.Open(finalPath)
				require.Nil(t, err)
				check_file, err := os.Stat(finalPath)
				require.Nil(t, err)
				require.True(t, check_file.Size() == tF.Size())
			}
		})
	}

}

func TestDeletePreCommitDir(t *testing.T) {

	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	allocID := randString(64)
	connID := randString(64)
	fileName := randString(5)
	remotePath := filepath.Join("/", randString(5)+".txt")
	alloc := &allocation{
		mu:    &sync.Mutex{},
		tmpMU: &sync.Mutex{},
	}

	fs.setAllocation(allocID, alloc)

	fPath := filepath.Join(fs.mp, randString(10)+".txt")
	size := 640 * KB
	validationRoot, fixedMerkleRoot, err := generateRandomData(fPath, int64(size))
	require.Nil(t, err)
	pathHash := encryption.Hash(remotePath)
	hasher := GetNewCommitHasher(int64(size))
	fid := &FileInputData{
		Name:            fileName,
		Path:            remotePath,
		ValidationRoot:  validationRoot,
		FixedMerkleRoot: fixedMerkleRoot,
		ChunkSize:       64 * KB,
		FilePathHash:    pathHash,
		Hasher:          hasher,
		Size:            int64(size),
	}
	// checkc if file to be uploaded exists
	f, err := os.Open(fPath)
	require.Nil(t, err)
	// Write file to temp location
	_, err = fs.WriteFile(allocID, connID, fid, f)
	require.Nil(t, err)
	f.Close()
	// check if file is written to temp location
	tempFilePath := fs.getTempPathForFile(allocID, fileName, pathHash, connID)
	tF, err := os.Stat(tempFilePath)
	require.Nil(t, err)
	nodeSize := getNodesSize(int64(size), util.MaxMerkleLeavesSize)
	require.Equal(t, int64(size), tF.Size()-nodeSize-FMTSize)
	seqPQ := seqpriorityqueue.NewSeqPriorityQueue()
	hasher.WG.Add(1)
	go hasher.Start(context.TODO(), connID, allocID, fileName, pathHash, seqPQ)
	seqPQ.Done(seqpriorityqueue.UploadData{
		Offset:    0,
		DataBytes: tF.Size(),
	})

	// Commit file to pre-commit location
	success, err := fs.CommitWrite(allocID, connID, fid)
	require.Nil(t, err)
	require.True(t, success)

	// Move data to final location
	err = fs.MoveToFilestore(allocID, validationRoot)
	require.Nil(t, err)
	prevValidationRoot := validationRoot

	validationRoot, fixedMerkleRoot, err = generateRandomData(fPath, int64(size))
	require.Nil(t, err)

	fid.ValidationRoot = validationRoot
	fid.FixedMerkleRoot = fixedMerkleRoot
	hasher = GetNewCommitHasher(int64(size))
	fid.Hasher = hasher

	// Write file to temp location
	f, err = os.Open(fPath)
	require.Nil(t, err)
	// Write file to temp location
	_, err = fs.WriteFile(allocID, connID, fid, f)
	require.Nil(t, err)
	f.Close()
	tempFilePath = fs.getTempPathForFile(allocID, fileName, pathHash, connID)
	_, err = os.Stat(tempFilePath)
	require.Nil(t, err)
	seqPQ = seqpriorityqueue.NewSeqPriorityQueue()
	hasher.WG.Add(1)
	go hasher.Start(context.TODO(), connID, allocID, fileName, pathHash, seqPQ)
	seqPQ.Done(seqpriorityqueue.UploadData{
		Offset:    0,
		DataBytes: tF.Size(),
	})

	success, err = fs.CommitWrite(allocID, connID, fid)
	require.Nil(t, err)
	require.True(t, success)

	preCommitPath := fs.getPreCommitPathForFile(allocID, validationRoot)
	_, err = os.Open(preCommitPath)
	require.Nil(t, err)

	err = fs.DeletePreCommitDir(allocID)
	require.Nil(t, err)

	preCommitPath = fs.getPreCommitPathForFile(allocID, validationRoot)
	_, err = os.Open(preCommitPath)
	require.NotNil(t, err)

	finalPath, err := fs.GetPathForFile(allocID, prevValidationRoot)
	require.Nil(t, err)
	_, err = os.Open(finalPath)
	require.Nil(t, err)

}

func TestStorageUploadUpdate(t *testing.T) {

	fs, cleanUp := setupStorage(t)
	defer cleanUp()

	allocID := randString(64)
	connID := randString(64)
	fileName := randString(5)
	remotePath := filepath.Join("/", randString(5)+".txt")
	alloc := &allocation{
		mu:    &sync.Mutex{},
		tmpMU: &sync.Mutex{},
	}

	fs.setAllocation(allocID, alloc)

	fPath := filepath.Join(fs.mp, randString(10)+".txt")
	size := 640 * KB
	validationRoot, fixedMerkleRoot, err := generateRandomData(fPath, int64(size))
	require.Nil(t, err)
	pathHash := encryption.Hash(remotePath)
	hasher := GetNewCommitHasher(int64(size))
	fid := &FileInputData{
		Name:            fileName,
		Path:            remotePath,
		ValidationRoot:  validationRoot,
		FixedMerkleRoot: fixedMerkleRoot,
		ChunkSize:       64 * KB,
		FilePathHash:    pathHash,
		Hasher:          hasher,
		Size:            int64(size),
	}
	// checkc if file to be uploaded exists
	f, err := os.Open(fPath)
	require.Nil(t, err)
	// Write file to temp location
	_, err = fs.WriteFile(allocID, connID, fid, f)
	require.Nil(t, err)
	f.Close()
	// check if file is written to temp location
	tempFilePath := fs.getTempPathForFile(allocID, fileName, pathHash, connID)
	tF, err := os.Stat(tempFilePath)
	require.Nil(t, err)
	nodeSize := getNodesSize(int64(size), util.MaxMerkleLeavesSize)
	require.Equal(t, int64(size), tF.Size()-nodeSize-FMTSize)
	seqPQ := seqpriorityqueue.NewSeqPriorityQueue()
	hasher.WG.Add(1)
	go hasher.Start(context.TODO(), connID, allocID, fileName, pathHash, seqPQ)
	seqPQ.Done(seqpriorityqueue.UploadData{
		Offset:    0,
		DataBytes: tF.Size(),
	})

	// Commit file to pre-commit location
	success, err := fs.CommitWrite(allocID, connID, fid)

	require.Nil(t, err)
	require.True(t, success)

	// Upload thumbnail
	thumbFileName := randString(5)
	size = 1687
	_, _, err = generateRandomData(fPath, int64(size))
	require.Nil(t, err)

	fid.Name = thumbFileName
	fid.IsThumbnail = true

	f, err = os.Open(fPath)
	require.Nil(t, err)
	// Write thumbnail file to temp location
	_, err = fs.WriteFile(allocID, connID, fid, f)
	f.Close()
	require.Nil(t, err)

	// check if thumbnail file is written to temp location
	tempFilePath = fs.getTempPathForFile(allocID, thumbFileName, pathHash, connID)
	finfo, err := os.Stat(tempFilePath)
	require.Nil(t, err)
	require.Equal(t, finfo.Size(), int64(size))

	// Check if the hash of the thumbnail file is same as the hash of the uploaded thumbnail file
	f, err = os.Open(tempFilePath)
	require.Nil(t, err)

	h := sha3.New256()
	_, err = io.Copy(h, f)
	require.Nil(t, err)
	pathWriter := fs.pathWriter(fid.Path)
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	f.Close()
	fid.ThumbnailHash = hex.EncodeToString(h.Sum(nil))
	prevThumbHash := fid.ThumbnailHash
	fid.Name = thumbFileName

	// Move data to final location
	err = fs.MoveToFilestore(allocID, validationRoot)
	require.Nil(t, err)

	// Commit thumbnail file to pre-commit location
	success, err = fs.CommitWrite(allocID, connID, fid)
	require.Nil(t, err)
	require.True(t, success)
	// Get the path of the pre-commit location of the thumbnail file and check if the file exists
	preCommitPath := fs.getPreCommitPathForFile(allocID, fid.ThumbnailHash)
	preFile, err := os.Open(preCommitPath)
	require.Nil(t, err)
	defer preFile.Close()
	check_file, err := os.Stat(preCommitPath)
	require.Nil(t, err)
	require.True(t, check_file.Size() == int64(size))

	// Update the thumbnail file
	_, _, err = generateRandomData(fPath, int64(size))
	require.Nil(t, err)

	f, err = os.Open(fPath)
	require.Nil(t, err)

	// Write thumbnail file to temp location
	_, err = fs.WriteFile(allocID, connID, fid, f)

	require.Nil(t, err)
	f.Close()
	// check if thumbnail file is written to temp location
	tempFilePath = fs.getTempPathForFile(allocID, thumbFileName, pathHash, connID)
	// Check if the hash of the thumbnail file is same as the hash of the updated thumbnail file
	f, err = os.Open(tempFilePath)
	require.Nil(t, err)

	h = sha3.New256()
	_, err = io.Copy(h, f)
	require.Nil(t, err)
	pathWriter = fs.pathWriter(fid.Path)
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	f.Close()
	fid.ThumbnailHash = hex.EncodeToString(h.Sum(nil))
	fid.IsThumbnail = false
	fid.Name = fileName

	// Move data to final location
	err = fs.MoveToFilestore(allocID, prevThumbHash)
	require.Nil(t, err)

	// Set fields to commit thumbnail file
	fid.IsThumbnail = true
	fid.Name = thumbFileName
	// Commit thumbnail file to pre-commit location
	success, err = fs.CommitWrite(allocID, connID, fid)
	require.Nil(t, err)
	require.True(t, success)

	// Get the path of the pre-commit location of the thumbnail file and check if the file exists
	preCommitPath = fs.getPreCommitPathForFile(allocID, fid.ThumbnailHash)

	preFile, err = os.Open(preCommitPath)

	require.Nil(t, err)
	defer preFile.Close()
	check_file, err = os.Stat(preCommitPath)
	require.Nil(t, err)
	fmt.Println("check_file.Size", check_file.Size())
	require.True(t, check_file.Size() == int64(size))

	h = sha3.New256()
	_, err = io.Copy(h, preFile)
	require.Nil(t, err)
	pathWriter = fs.pathWriter(fid.Path)
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	require.Equal(t, hex.EncodeToString(h.Sum(nil)), fid.ThumbnailHash)

	input := &ReadBlockInput{
		AllocationID: allocID,
		FileSize:     int64(size),
		Hash:         fid.ThumbnailHash,
		IsThumbnail:  true,
		NumBlocks:    1,
		IsPrecommit:  true,
	}

	data, err := fs.GetFileBlock(input)
	require.Nil(t, err)
	require.Equal(t, size, len(data.Data))
	h = sha3.New256()
	buf := bytes.NewReader(data.Data)
	_, err = io.Copy(h, buf)
	require.Nil(t, err)
	pathWriter = fs.pathWriter(fid.Path)
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	require.Equal(t, hex.EncodeToString(h.Sum(nil)), fid.ThumbnailHash)
	fPath, err = fs.GetPathForFile(allocID, prevThumbHash)
	require.Nil(t, err)
	f, err = os.Open(fPath)
	require.Nil(t, err)
	h = sha3.New256()
	_, err = io.Copy(h, f)
	require.Nil(t, err)
	pathWriter = fs.pathWriter(fid.Path)
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	require.Equal(t, hex.EncodeToString(h.Sum(nil)), prevThumbHash)
	f.Close()
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

	permanentFPath := fs.getPreCommitPathForFile(allocID, validationRoot)
	// require.Nil(t, err)

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
		fileName         string
		remotePath       string
	}

	tests := []input{
		{
			testName:       "start block less than 0",
			blockNum:       -1,
			numBlocks:      10,
			validationRoot: validationRoot,
			expectedError:  true,
			errorContains:  "invalid_block_number",
			fileName:       "hello",
			remotePath:     fPath,
		},
		{
			testName:       "start block greater than max block num",
			blockNum:       20,
			numBlocks:      10,
			validationRoot: validationRoot,
			expectedError:  true,
			errorContains:  "invalid_block_number",
			fileName:       "hello",
			remotePath:     fPath,
		}, {
			testName:       "Non-existing file",
			blockNum:       1,
			numBlocks:      10,
			validationRoot: randString(64),
			expectedError:  true,
			errorContains:  "no such file or directory",
			fileName:       "hello",
			remotePath:     randString(20),
		},
		{
			testName:         "successful response",
			blockNum:         0,
			numBlocks:        10,
			expectedDataSize: int64(size),
			validationRoot:   validationRoot,
			fileName:         "hello",
			remotePath:       fPath,
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
				IsPrecommit:   true,
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
	fPath := fs.getPreCommitPathForFile(allocID, validationRoot)

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
				IsPrecommit:  true,
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
				LeafHash: encryption.ShaHash(challengeProof.Data),
				RootHash: rootHash,
				Nodes:    challengeProof.Proof,
				LeafInd:  test.blockOffset,
			}

			require.True(t, fmp.VerifyMerklePath())
		})
	}
}

func TestValidationRoot(t *testing.T) {

	thumbnailBytes, _ := base64.StdEncoding.DecodeString(`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`)
	size := int64(3374)

	fs, cleanUp := setupStorage(t)
	defer cleanUp()
	fPath := filepath.Join(fs.mp, randString(10)+".txt")
	cH := GetNewCommitHasher(size)
	_, err := cH.Write(thumbnailBytes)
	require.Nil(t, err)

	err = cH.Finalize()
	require.Nil(t, err)

	f, err := os.Create(fPath)
	require.Nil(t, err)
	defer f.Close()

	validationMerkleRoot, err := cH.vt.CalculateRootAndStoreNodes(f)
	require.Nil(t, err)

	bufReader := bytes.NewReader(thumbnailBytes)
	pathWriter := fs.pathWriter(fPath)
	h := sha3.New256()
	_, err = io.Copy(h, pathWriter)
	require.Nil(t, err)
	_, err = io.Copy(h, bufReader)
	require.Nil(t, err)

	hash := hex.EncodeToString(h.Sum(nil))
	require.NotEqual(t, hash, hex.EncodeToString(validationMerkleRoot))
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
	SetFileStore(&fs)

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
				if n == 0 {
					break
				}
				goto final
			}
			return
		}
		if n != 64*KB {
			fmt.Println("n is ", n)
			return "", errors.New("invalid byte length. Must be 64 KB")
		}

		_, err = fixedMT.Write(b)
		if err != nil {
			return
		}

		count++
		continue
	final:
		if n != 64*KB {
			return "", errors.New("invalid byte length. Must be 64 KB")
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
