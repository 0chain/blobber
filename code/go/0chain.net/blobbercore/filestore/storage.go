package filestore

// File management i.e. evenly distributing files so that OS takes less time to process lookups, is tricky. One might think
// of creating multiple indexes, 0.......unlimited, but this will result in unlimited number of directories is base path.
//
// base path --> where you'd want to store all the files with some file management techniques.
//
// Using multiple indexes is manageable if allocation size would be fixed during its life time, but its not. It can decrease and
// increase. Also the blobber's size can increase or decrease. Thus, managing files using numerical indexes will be complex.
//
// To deal with it, we can use contentHash of some file so that files are distributed randomly. Randomness seems to distribute files
// close to evenly. So if we have an lookup hash of a file "4c9bad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56" then we
// can store this file in following directory:
// `base_path/{allocation_id}/4/c/9/b/a/d252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56`
// With above structure, an allocation can have upto 16*16*16*16*16 = 1048576 directories for storing files and 16 + 16^2+16^3+16^4 = 69904
// for parent directories of 1048576 directories.
//
// If some direcotry would contain 1000 files then total files stored by an allocation would be 1048576*1000 = 1048576000, around 1 billions
// file.
// Blobber should choose this level calculating its size and number of files its going to store and increase/decrease levels of directories.
//
// Above situation also occurs to store {allocation_id} as base directory for its files when blobber can have thousands of allocations.
// We can also use allocation_id to distribute allocations.
// For allocation using 3 levels we would have 16*16*16 = 4096 unique directories, Each directory can contain 1000 allocations. Thus storing
// 4096000 number of allocations.
//

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/gosdk/core/util"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
	"golang.org/x/sys/unix"
)

const (
	TempDir         = "tmp"
	MerkleChunkSize = 64
	ChunkSize       = 64 * KB
)

func (fs *FileStore) WriteFile(allocID, conID string, fileData *FileInputData, infile multipart.File) (*FileOutputData, error) {
	tempFilePath := fs.getTempPathForFile(allocID, fileData.Name, encryption.Hash(fileData.Path), conID)
	var initialSize int64
	finfo, err := os.Stat(tempFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, common.NewError("file_stat_error", err.Error())
	}
	if finfo != nil {
		initialSize = finfo.Size()
	}

	if err = createDirs(filepath.Dir(tempFilePath)); err != nil {
		return nil, common.NewError("dir_creation_error", err.Error())
	}

	f, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, common.NewError("file_open_error", err.Error())
	}
	defer f.Close()

	_, err = f.Seek(fileData.UploadOffset, io.SeekStart)
	if err != nil {
		return nil, common.NewError("file_seek_error", err.Error())
	}

	writtenSize, err := io.Copy(f, infile)
	if err != nil {
		return nil, common.NewError("file_write_error", err.Error())
	}

	finfo, err = f.Stat()
	if err != nil {
		return nil, common.NewError("file_stat_error", err.Error())
	}

	fileRef := &FileOutputData{}

	currentSize := finfo.Size()
	if currentSize > initialSize { // Is chunk new or rewritten
		fileRef.ChunkUploaded = true
		fs.updateAllocTempFileSize(allocID, currentSize-initialSize)
	}

	fileRef.Size = writtenSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path

	return fileRef, nil
}

func (fs *FileStore) CommitWrite(allocID, conID string, fileData *FileInputData) (bool, error) {

	tempFilePath := fs.getTempPathForFile(allocID, fileData.Name, encryption.Hash(fileData.Path), conID)
	r, err := os.Open(tempFilePath)
	if err != nil {
		return false, err
	}

	var fPath string
	defer func() {
		if err != nil {
			os.Remove(fPath)
		} else {
			os.Remove(tempFilePath)
		}
	}()
	defer r.Close()

	if fileData.IsThumbnail {
		h := sha3.New256()
		_, err = io.Copy(h, r)
		if err != nil {
			return false, common.NewError("read_error", err.Error())
		}
		hash := hex.EncodeToString(h.Sum(nil))
		if hash != fileData.ThumbnailHash {
			return false, common.NewError("hash_mismatch", "calculated thumbnail hash does not match with expected hash")
		}

		fPath, err = fs.GetPathForFile(allocID, hash)
		if err != nil {
			return false, common.NewError("get_file_path_error", err.Error())
		}
		err = createDirs(filepath.Dir(fPath))
		if err != nil {
			return false, common.NewError("blob_object_dir_creation_error", err.Error())
		}

		f, err := os.Create(fPath)
		if err != nil {
			return false, err
		}
		defer f.Close()
		_, err = r.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}

		_, err = io.Copy(f, r)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	key := getKey(allocID, fileData.ValidationRoot)
	l, _ := contentHashMapLock.GetLock(key)
	l.Lock()
	defer func() {
		if err != nil {
			l.Unlock()
		}
	}()

	fPath, err = fs.GetPathForFile(allocID, fileData.ValidationRoot)
	if err != nil {
		return false, common.NewError("get_file_path_error", err.Error())
	}

	err = createDirs(filepath.Dir(fPath))
	if err != nil {
		return false, common.NewError("blob_object_dir_creation_error", err.Error())
	}

	f, err := os.Create(fPath)
	if err != nil {
		return false, err
	}

	defer r.Close()

	fStat, err := r.Stat()
	if err != nil {
		return false, common.NewError("stat_error", err.Error())
	}

	fileSize := fStat.Size()
	hasher := GetNewCommitHasher(fileSize)
	_, err = io.Copy(hasher, r)
	if err != nil {
		return false, common.NewError("read_write_error", err.Error())
	}

	err = hasher.Finalize()
	if err != nil {
		return false, common.NewError("finalize_error", err.Error())
	}

	fmtRootBytes, err := hasher.fmt.CalculateRootAndStoreNodes(f)
	if err != nil {
		return false, common.NewError("fmt_hash_calculation_error", err.Error())
	}

	validationRootBytes, err := hasher.vt.CalculateRootAndStoreNodes(f)
	if err != nil {
		return false, common.NewError("validation_hash_calculation_error", err.Error())
	}

	fmtRoot := hex.EncodeToString(fmtRootBytes)
	validationRoot := hex.EncodeToString(validationRootBytes)

	if fmtRoot != fileData.FixedMerkleRoot {
		return false, common.NewError("fixed_merkle_root_mismatch",
			fmt.Sprintf("Expected %s got %s", fileData.FixedMerkleRoot, fmtRoot))
	}

	if validationRoot != fileData.ValidationRoot {
		return false, common.NewError("validation_root_mismatch",
			"calculated validation root does not match with client's validation root")
	}

	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return false, common.NewError("seek_error", err.Error())
	}

	_, err = io.Copy(f, r)
	if err != nil {
		return false, common.NewError("write_error", err.Error())
	}

	f.Close()
	l.Unlock()

	fs.updateAllocTempFileSize(allocID, -fileSize)
	// Each commit write should add 1 to file number because of the following:
	// 1. NewFile: Obvioulsy needs to increment by 1
	// 2. UpdateFile: First it will delete, decrements file number by 1 and will Call CommitWrite
	// 3. Rename: Doesn't call CommitWrite i.e. doesn't do anything with file data
	// 4. Copy: Doesn't call CommitWrite. Same as Rename
	// 5. Move: It is Copy + Delete. Delete will not delete file if ref exists in database. i.e. copy would create
	// ref that refers to this file therefore it will be skipped
	fs.incrDecrAllocFileSizeAndNumber(allocID, fileSize, 1)
	return true, nil
}

func (fs *FileStore) DeleteFile(allocID, contentHash string) error {
	fileObjectPath, err := fs.GetPathForFile(allocID, contentHash)
	if err != nil {
		return common.NewError("get_file_path_error", err.Error())
	}

	finfo, err := os.Stat(fileObjectPath)
	if err != nil {
		return err
	}
	size := finfo.Size()

	key := getKey(allocID, contentHash)

	// isNew is checked if a fresh lock is acquired. If lock is just holded by this process then it will actually delete
	// the file.
	// If isNew is false, either same content is being written or deleted. Therefore, this process can rely on other process
	// to either keep or delete file
	l, isNew := contentHashMapLock.GetLock(key)
	if !isNew {
		fs.incrDecrAllocFileSizeAndNumber(allocID, -size, -1)

		return common.NewError("not_new_lock",
			fmt.Sprintf("lock is acquired by other process to process on content. allocation id: %s content hash: %s",
				allocID, contentHash))
	}
	l.Lock()
	defer l.Unlock()

	err = os.Remove(fileObjectPath)
	if err != nil {
		return err
	}

	fs.incrDecrAllocFileSizeAndNumber(allocID, -size, -1)
	return nil
}

func (fs *FileStore) DeleteTempFile(allocID, conID string, fd *FileInputData) error {
	fileObjectPath := fs.getTempPathForFile(allocID, fd.Name, encryption.Hash(fd.Path), conID)

	finfo, err := os.Stat(fileObjectPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	size := finfo.Size()
	err = os.Remove(fileObjectPath)
	if err != nil {
		logging.Logger.Warn("invalid_path", zap.String("fileObjectPath", fileObjectPath), zap.Error(err))
		return err
	}

	fs.updateAllocTempFileSize(allocID, -size)

	return nil
}

// GetFileBlock Get blocks of file starting from blockNum upto numBlocks. blockNum can't be less than 1.
func (fs *FileStore) GetFileBlock(readBlockIn *ReadBlockInput) (*FileDownloadResponse, error) {

	// todo also differentiate thumbnail and actual file
	startBlock := readBlockIn.StartBlockNum
	endBlock := readBlockIn.StartBlockNum + readBlockIn.NumBlocks

	if startBlock < 0 {
		return nil, common.NewError("invalid_block_number", "Invalid block number. Start block number cannot be negative")
	}

	fileObjectPath, err := fs.GetPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
	if err != nil {
		return nil, common.NewError("get_file_path_error", err.Error())
	}

	file, err := os.Open(fileObjectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	filesize := readBlockIn.FileSize
	maxBlockNum := int64(math.Ceil(float64(filesize) / ChunkSize))

	if int64(startBlock) > maxBlockNum {
		return nil, common.NewError("invalid_block_number",
			fmt.Sprintf("Invalid block number. Start block %d is greater than maximum blocks %d",
				startBlock, maxBlockNum))
	}

	vp := validationTreeProof{
		dataSize: readBlockIn.FileSize,
	}

	nodes, indexes, nodesSize, err := vp.GetMerkleProofOfMultipleIndexes(file, startBlock, endBlock)
	if err != nil {
		return nil, common.NewError("get_merkle_proof_error", err.Error())
	}

	vmp := &FileDownloadResponse{
		Nodes:   nodes,
		Indexes: indexes,
	}

	fileOffset := nodesSize + int64(startBlock)*ChunkSize

	_, err = file.Seek(fileOffset, io.SeekStart)
	if err != nil {
		return nil, common.NewError("seek_error", err.Error())
	}

	buffer := make([]byte, readBlockIn.NumBlocks*ChunkSize)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	vmp.Data = buffer[:n]
	return vmp, nil
}

func (fs *FileStore) GetBlocksMerkleTreeForChallenge(in *ChallengeReadBlockInput) (*ChallengeResponse, error) {

	if in.BlockOffset < 0 || in.BlockOffset >= util.FixedMerkleLeaves {
		return nil, common.NewError("invalid_block_number", "Invalid block offset")
	}

	fileObjectPath, err := fs.GetPathForFile(in.AllocationID, in.Hash)
	if err != nil {
		return nil, common.NewError("get_file_path_error", err.Error())
	}

	file, err := os.Open(fileObjectPath)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	fmp := &fixedMerkleTreeProof{
		idx:      in.BlockOffset,
		dataSize: in.FileSize,
	}

	_, err = file.Seek(-in.FileSize, io.SeekEnd)
	if err != nil {
		return nil, common.NewError("seek_error", err.Error())
	}
	merkleProof, err := fmp.GetMerkleProof(file)
	if err != nil {
		return nil, common.NewError("get_merkle_proof_error", err.Error())
	}

	proofByte, err := fmp.GetLeafContent(file)
	if err != nil {
		return nil, common.NewError("get_leaf_content_error", err.Error())
	}

	return &ChallengeResponse{
		Proof:   merkleProof,
		Data:    proofByte,
		LeafInd: in.BlockOffset,
	}, nil
}

func (fs FileStore) GetCurrentDiskCapacity() uint64 {
	return fs.diskCapacity
}

func (fs *FileStore) CalculateCurrentDiskCapacity() error {

	var volStat unix.Statfs_t
	err := unix.Statfs(fs.mp, &volStat)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return err
	}

	fs.diskCapacity = volStat.Bavail * uint64(volStat.Bsize)
	return nil
}

func (fs *FileStore) isMountPoint() bool {
	if !filepath.IsAbs(fs.mp) {
		logging.Logger.Error(fmt.Sprintf("%s is not absolute path", fs.mp))
		return false
	}

	/*Below code is temporary fix unless devops comes with exact mountpoint*/
	if err := os.MkdirAll(fs.mp, 0777); err != nil {
		logging.Logger.Error(fmt.Sprintf("Error %s while creating directories", err.Error()))
		return false
	}
	if true {
		return true
	}
	/*Above code is temporary fix unless devops comes with exact mountpoint*/

	realMP, err := filepath.EvalSymlinks(fs.mp)
	if err != nil {
		logging.Logger.Error(err.Error())
		return false
	}

	finfo, err := os.Lstat(realMP)
	if err != nil {
		logging.Logger.Error(err.Error())
		return false
	}

	pinfo, err := os.Lstat(filepath.Dir(realMP))
	if err != nil {
		logging.Logger.Error(err.Error())
		return false
	}

	dev := finfo.Sys().(*syscall.Stat_t).Dev
	pDev := pinfo.Sys().(*syscall.Stat_t).Dev

	return dev != pDev
}

func (fstr *FileStore) getTemporaryStorageDetails(
	ctx context.Context, a *allocation, ID string, ch <-chan struct{}, wg *sync.WaitGroup) {

	defer func() {
		wg.Done()
		<-ch
	}()

	var err error
	defer func() {
		if err != nil {
			panic(err)
		}
	}()

	tempDir := fstr.getAllocTempDir(ID)

	finfo, err := os.Stat(tempDir)
	if errors.Is(err, os.ErrNotExist) {
		err = nil
		return
	} else if err != nil {
		return
	}

	if !finfo.IsDir() {
		err = fmt.Errorf("path %s is of type file", tempDir)
		return
	}

	var totalSize uint64
	err = filepath.Walk(tempDir, func(path string, info fs.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		totalSize += uint64(info.Size())
		return nil
	})

	if err != nil {
		return
	}

	a.tmpMU.Lock()
	defer a.tmpMU.Unlock()

	a.tmpFileSize = totalSize
}

func (fs *FileStore) getAllocDir(allocID string) string {
	return filepath.Join(fs.mp, getPartialPath(allocID, getDirLevelsForAllocations()))
}

func (fs *FileStore) GetPathForFile(allocID, hash string) (string, error) {
	if len(allocID) != 64 || len(hash) != 64 {
		return "", errors.New("length of allocationID/hash must be 64")
	}

	return filepath.Join(fs.getAllocDir(allocID), getPartialPath(hash, getDirLevelsForFiles())), nil
}

// getPath returns "/" separated strings with the given levels.
func getPartialPath(hash string, levels []int) string {
	var count int
	var pStr []string
	for _, i := range levels {
		pStr = append(pStr, hash[count:count+i])
		count += i
	}
	pStr = append(pStr, hash[count:])
	return strings.Join(pStr, "/")
}

/*****************************************Temporary files management*****************************************/

func (fs *FileStore) getAllocTempDir(allocID string) string {
	return filepath.Join(fs.getAllocDir(allocID), TempDir)
}

func (fs *FileStore) getTempPathForFile(allocId, fileName, pathHash, connectionID string) string {
	return filepath.Join(fs.getAllocTempDir(allocId), fileName+"."+pathHash+"."+connectionID)
}

func (fs *FileStore) updateAllocTempFileSize(allocID string, size int64) {
	alloc := fs.getAllocation(allocID)
	if alloc == nil {
		return
	}

	alloc.tmpMU.Lock()
	defer alloc.tmpMU.Unlock()

	alloc.tmpFileSize += uint64(size)
}

// GetTempFilesSizeOfAllocation Get total file sizes of all allocation that are not yet committed
func (fs *FileStore) GetTotalTempFileSizes() (s uint64) {
	for _, alloc := range fs.mAllocs {
		s += alloc.tmpFileSize
	}
	return
}

func (fs *FileStore) GetTempFilesSizeOfAllocation(allocID string) uint64 {
	alloc := fs.getAllocation(allocID)
	if alloc != nil {
		return alloc.tmpFileSize
	}
	return 0
}

// GetTotalCommittedFileSize Get total committed file sizes of all allocations
func (fs *FileStore) GetTotalCommittedFileSize() (s uint64) {
	for _, alloc := range fs.mAllocs {
		s += alloc.filesSize
	}
	return
}

func (fs *FileStore) GetCommittedFileSizeOfAllocation(allocID string) uint64 {
	alloc := fs.getAllocation(allocID)
	if alloc != nil {
		return alloc.filesSize
	}
	return 0
}

// GetTotalFilesSize Get total file sizes of all allocations; committed or not committed
func (fs *FileStore) GetTotalFilesSize() (s uint64) {
	for _, alloc := range fs.mAllocs {
		s += alloc.filesSize + alloc.tmpFileSize
	}
	return
}

// GetTotalFilesSize Get total file sizes of an allocation; committed or not committed
func (fs *FileStore) GetTotalFilesSizeOfAllocation(allocID string) uint64 {
	alloc := fs.getAllocation(allocID)
	if alloc != nil {
		return alloc.filesSize + alloc.tmpFileSize
	}
	return 0
}

/***************************************************Misc***************************************************/

func createDirs(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}
