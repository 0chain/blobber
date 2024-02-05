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
	"bytes"
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
	"time"

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
	PreCommitDir    = "precommit"
	MerkleChunkSize = 64
	ChunkSize       = 64 * KB
	BufferSize      = 80 * ChunkSize
)

func (fs *FileStore) WriteFile(allocID, conID string, fileData *FileInputData, infile multipart.File) (*FileOutputData, error) {
	tempFilePath := fs.getTempPathForFile(allocID, fileData.Name, fileData.FilePathHash, conID)
	var (
		initialSize int64
		nodeSize    int64
		offset      int64
	)
	finfo, err := os.Stat(tempFilePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, common.NewError("file_stat_error", err.Error())
	}
	if finfo != nil {
		initialSize = finfo.Size()
	}
	if !fileData.IsThumbnail {
		nodeSize = getNodesSize(fileData.Size, util.MaxMerkleLeavesSize)
		offset = fileData.UploadOffset + nodeSize + FMTSize
	}
	if err = createDirs(filepath.Dir(tempFilePath)); err != nil {
		return nil, common.NewError("dir_creation_error", err.Error())
	}
	f, err := os.OpenFile(tempFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, common.NewError("file_open_error", err.Error())
	}
	defer f.Close()

	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, common.NewError("file_seek_error", err.Error())
	}
	buf := make([]byte, BufferSize)
	writtenSize, err := io.CopyBuffer(f, infile, buf)
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
		fs.updateAllocTempFileSize(allocID, currentSize-initialSize)
	}
	if currentSize > fileData.Size+nodeSize+FMTSize {
		_ = os.Remove(tempFilePath)
		return nil, common.NewError("file_size_mismatch", "File size is greater than expected")
	}
	logging.Logger.Info("temp_file_write: ", zap.String("filePath", fileData.Path), zap.Int64("currentSize", currentSize), zap.Int64("initialSize", initialSize), zap.Int64("writtenSize", writtenSize), zap.Int64("offset", fileData.UploadOffset), zap.Bool("ChunkUploaded", fileRef.ChunkUploaded))
	fileRef.Size = writtenSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path
	fileRef.ContentSize = currentSize - nodeSize - FMTSize
	return fileRef, nil
}

func (fs *FileStore) WriteDataToTree(allocID, connID, fileName, filePathHash string, hasher *CommitHasher) error {
	tempFilePath := fs.getTempPathForFile(allocID, fileName, filePathHash, connID)

	offset := getNodesSize(hasher.dataSize, util.MaxMerkleLeavesSize) + FMTSize
	f, err := os.Open(tempFilePath)
	if err != nil {
		return common.NewError("file_open_error", err.Error())
	}
	defer f.Close()
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return common.NewError("file_seek_error", err.Error())
	}
	bufSize := BufferSize * 5
	if int64(bufSize) > hasher.dataSize {
		bufSize = int(hasher.dataSize)
	}
	buf := make([]byte, bufSize)
	_, err = io.CopyBuffer(hasher, f, buf)
	if err != nil {
		return common.NewError("hasher_write_error", err.Error())
	}
	return nil
}

func (fs *FileStore) GetTempFilePath(allocID, connID, fileName, filePathHash string) string {
	return fs.getTempPathForFile(allocID, fileName, filePathHash, connID)
}

func (fs *FileStore) MoveToFilestore(allocID, hash string) error {

	fPath, err := fs.GetPathForFile(allocID, hash)
	if err != nil {
		return common.NewError("get_file_path_error", err.Error())
	}

	preCommitPath := fs.getPreCommitPathForFile(allocID, hash)

	err = createDirs(filepath.Dir(fPath))
	if err != nil {
		return common.NewError("blob_object_dir_creation_error", err.Error())
	}

	_ = os.Rename(preCommitPath, fPath)
	return nil
}

func (fs *FileStore) DeleteFromFilestore(allocID, hash string) error {

	fPath, err := fs.GetPathForFile(allocID, hash)
	if err != nil {
		return common.NewError("get_file_path_error", err.Error())
	}
	logging.Logger.Info("Deleting file from filestore", zap.String("path", fPath))
	err = os.Remove(fPath)
	if err != nil {
		return common.NewError("blob_object_dir_creation_error", err.Error())
	}

	return nil
}

func (fs *FileStore) DeletePreCommitDir(allocID string) error {

	preCommitDir := fs.getPreCommitDir(allocID)
	err := os.RemoveAll(preCommitDir)
	if err != nil {
		return common.NewError("pre_commit_dir_deletion_error", err.Error())
	}
	tempDir := fs.getAllocTempDir(allocID)
	err = os.RemoveAll(tempDir)
	if err != nil {
		return common.NewError("temp_dir_deletion_error", err.Error())
	}

	return nil
}

func (fs *FileStore) CommitWrite(allocID, conID string, fileData *FileInputData) (bool, error) {

	logging.Logger.Info("Committing write", zap.String("allocation_id", allocID), zap.Any("file_data", fileData))
	filePathHash := encryption.Hash(fileData.Path)
	tempFilePath := fs.getTempPathForFile(allocID, fileData.Name, filePathHash, conID)

	fileHash := fileData.ValidationRoot
	if fileData.IsThumbnail {
		fileHash = fileData.ThumbnailHash
	}

	preCommitPath := fs.getPreCommitPathForFile(allocID, fileHash)

	err := createDirs(filepath.Dir(preCommitPath))
	if err != nil {
		return false, common.NewError("blob_object_precommit_dir_creation_error", err.Error())
	}

	r, err := os.OpenFile(tempFilePath, os.O_RDWR, 0644)
	if err != nil {
		return false, err
	}

	defer func() {
		r.Close()
		if err != nil {
			os.Remove(preCommitPath)
		} else {
			os.Remove(tempFilePath)
		}
	}()

	if fileData.IsThumbnail {

		h := sha3.New256()
		_, err = io.Copy(h, r)
		if err != nil {
			return false, common.NewError("read_error", err.Error())
		}
		_, err = h.Write([]byte(fileData.Path))
		if err != nil {
			return false, common.NewError("read_error", err.Error())
		}
		hash := hex.EncodeToString(h.Sum(nil))
		if hash != fileData.ThumbnailHash {
			return false, common.NewError("hash_mismatch",
				fmt.Sprintf("calculated thumbnail hash does not match with expected hash. Expected %s, got %s.",
					fileData.ThumbnailHash, hash))
		}

		_, err = r.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}

		err = os.Rename(tempFilePath, preCommitPath)
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

	rStat, err := r.Stat()
	if err != nil {
		return false, common.NewError("stat_error", err.Error())
	}
	nodeSie := getNodesSize(fileData.Size, util.MaxMerkleLeavesSize)
	fileSize := rStat.Size() - nodeSie - FMTSize
	now := time.Now()
	err = fileData.Hasher.Wait(conID, allocID, fileData.Name, filePathHash)
	if err != nil {
		return false, common.NewError("hasher_wait_error", err.Error())
	}
	elapsedWait := time.Since(now)
	fmtRootBytes, err := fileData.Hasher.fmt.CalculateRootAndStoreNodes(r)
	if err != nil {
		return false, common.NewError("fmt_hash_calculation_error", err.Error())
	}

	validationRootBytes, err := fileData.Hasher.vt.CalculateRootAndStoreNodes(r)
	if err != nil {
		return false, common.NewError("validation_hash_calculation_error", err.Error())
	}
	fmtRoot := hex.EncodeToString(fmtRootBytes)
	validationRoot := hex.EncodeToString(validationRootBytes)
	elapsedRoot := time.Since(now) - elapsedWait
	if fmtRoot != fileData.FixedMerkleRoot {
		return false, common.NewError("fixed_merkle_root_mismatch",
			fmt.Sprintf("Expected %s got %s", fileData.FixedMerkleRoot, fmtRoot))
	}
	if validationRoot != fileData.ValidationRoot {
		return false, common.NewError("validation_root_mismatch",
			"calculated validation root does not match with client's validation root")
	}

	err = os.Rename(tempFilePath, preCommitPath)
	if err != nil {
		return false, common.NewError("write_error", err.Error())
	}

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
	logging.Logger.Info("Committing write done", zap.String("file_path", fileData.Path), zap.Duration("elapsed_wait", elapsedWait), zap.Duration("elapsed_root", elapsedRoot), zap.Duration("elapsed_total", time.Since(now)))
	return true, nil
}

func (fs *FileStore) GetFilePathSize(allocID, filehash, thumbHash string) (int64, int64, error) {

	filePath, err := fs.GetPathForFile(allocID, filehash)

	if err != nil {
		return 0, 0, err
	}

	stat, err := os.Stat(filePath)

	fileSize := stat.Size()

	if err != nil {
		return 0, 0, err
	}
	var thumbSize int64
	if thumbHash != "" {
		thumbPath, err := fs.GetPathForFile(allocID, thumbHash)
		if err != nil {
			return 0, 0, err
		}
		stat, err := os.Stat(thumbPath)
		if err != nil {
			return 0, 0, err
		}
		thumbSize = stat.Size()
	}

	return fileSize, thumbSize, nil

}

// Only decreasing the file size and number. Not deleting the file
func (fs *FileStore) DeleteFile(allocID, validationRoot string) error {

	fileObjectPath, err := fs.GetPathForFile(allocID, validationRoot)
	if err != nil {
		return err
	}

	finfo, err := os.Stat(fileObjectPath)
	if err != nil {
		return err
	}

	size := finfo.Size()

	key := getKey(allocID, validationRoot)

	// isNew is checked if a fresh lock is acquired. If lock is just holded by this process then it will actually delete
	// the file.
	// If isNew is false, either same content is being written or deleted. Therefore, this process can rely on other process
	// to either keep or delete file
	l, isNew := contentHashMapLock.GetLock(key)
	if !isNew {

		fs.incrDecrAllocFileSizeAndNumber(allocID, -size, -1)

		return common.NewError("not_new_lock",
			fmt.Sprintf("lock is acquired by other process to process on content. allocation id: %s content hash: %s",
				allocID, validationRoot))
	}
	l.Lock()
	defer l.Unlock()

	fs.incrDecrAllocFileSizeAndNumber(allocID, -size, -1)

	return nil
}

func (fs *FileStore) DeleteTempFile(allocID, conID string, fd *FileInputData) error {
	if allocID == "" {
		logging.Logger.Error("invalid_allocation_id", zap.String("connection_id", conID), zap.Any("file_data", fd))
		return common.NewError("invalid_allocation_id", "Allocation id cannot be empty")
	}
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

func (fs *FileStore) DeleteAllocation(allocID string) {
	preCommitDir := fs.getPreCommitDir(allocID)
	_ = os.RemoveAll(preCommitDir)
	tempDir := fs.getAllocTempDir(allocID)
	_ = os.RemoveAll(tempDir)
	alloDir := fs.getAllocDir(allocID)
	_ = os.RemoveAll(alloDir)
	fs.removeAllocation(allocID)
}

func (fs *FileStore) GetFileThumbnail(readBlockIn *ReadBlockInput) (*FileDownloadResponse, error) {
	var fileObjectPath string
	var err error
	startBlock := readBlockIn.StartBlockNum
	if startBlock < 0 {
		return nil, common.NewError("invalid_block_number", "Invalid block number. Start block number cannot be negative")
	}
	if readBlockIn.IsPrecommit {
		fileObjectPath = fs.getPreCommitPathForFile(readBlockIn.AllocationID,
			readBlockIn.Hash)
	} else {
		fileObjectPath, err = fs.GetPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
		if err != nil {
			return nil, common.NewError("get_file_path_error", err.Error())
		}
	}

	file, err := os.Open(fileObjectPath)
	if err != nil {
		if readBlockIn.IsPrecommit {
			fileObjectPath, err = fs.GetPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
			if err != nil {
				return nil, common.NewError("get_file_path_error", err.Error())
			}
			file, err = os.Open(fileObjectPath)
			if err != nil {
				return nil, common.NewError("read_error", err.Error())
			}
		} else {
			return nil, common.NewError("read_error", err.Error())
		}
	}
	defer file.Close()

	if readBlockIn.VerifyDownload {
		h := sha3.New256()
		_, err = io.Copy(h, file)
		if err != nil {
			return nil, common.NewError("read_error", err.Error())
		}
		hash := hex.EncodeToString(h.Sum(nil))

		if hash != readBlockIn.Hash {
			return nil, common.NewError("hash_mismatch", fmt.Sprintf("Hash mismatch. Expected %s, got %s", readBlockIn.Hash, hash))
		}

	}
	filesize := readBlockIn.FileSize
	maxBlockNum := int64(math.Ceil(float64(filesize) / ChunkSize))

	if int64(startBlock) > maxBlockNum {
		return nil, common.NewError("invalid_block_number",
			fmt.Sprintf("Invalid block number. Start block %d is greater than maximum blocks %d",
				startBlock, maxBlockNum))
	}

	fileOffset := int64(startBlock) * ChunkSize
	_, err = file.Seek(fileOffset, io.SeekStart)
	if err != nil {
		return nil, common.NewError("seek_error", err.Error())
	}

	buffer := make([]byte, readBlockIn.NumBlocks*ChunkSize)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return &FileDownloadResponse{
		Data: buffer[:n],
	}, nil
}

// GetFileBlock Get blocks of file starting from blockNum upto numBlocks. blockNum can't be less than 1.
func (fs *FileStore) GetFileBlock(readBlockIn *ReadBlockInput) (*FileDownloadResponse, error) {
	var fileObjectPath string
	var err error

	if readBlockIn.IsThumbnail {
		return fs.GetFileThumbnail(readBlockIn)
	}

	startBlock := readBlockIn.StartBlockNum
	endBlock := readBlockIn.StartBlockNum + readBlockIn.NumBlocks - 1

	if startBlock < 0 {
		return nil, common.NewError("invalid_block_number", "Invalid block number. Start block number cannot be negative")
	}
	if readBlockIn.IsPrecommit {
		fileObjectPath = fs.getPreCommitPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
	} else {
		fileObjectPath, err = fs.GetPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
		if err != nil {
			return nil, common.NewError("get_file_path_error", err.Error())
		}
	}
	file, err := os.Open(fileObjectPath)
	if err != nil {
		if readBlockIn.IsPrecommit {
			fileObjectPath, err = fs.GetPathForFile(readBlockIn.AllocationID, readBlockIn.Hash)
			if err != nil {
				return nil, common.NewError("get_file_path_error", err.Error())
			}
			file, err = os.Open(fileObjectPath)
			if err != nil {
				return nil, common.NewError("read_error", err.Error())
			}
		} else {
			return nil, err
		}
	}
	defer file.Close()

	filesize := readBlockIn.FileSize
	maxBlockNum := int64(math.Ceil(float64(filesize) / ChunkSize))

	if int64(startBlock) > maxBlockNum {
		return nil, common.NewError("invalid_block_number",
			fmt.Sprintf("Invalid block number. Start block %d is greater than maximum blocks %d",
				startBlock, maxBlockNum))
	}

	nodesSize := getNodesSize(filesize, util.MaxMerkleLeavesSize)
	vmp := &FileDownloadResponse{}

	if readBlockIn.VerifyDownload {
		vp := validationTreeProof{
			dataSize: readBlockIn.FileSize,
		}

		logging.Logger.Debug("calling GetMerkleProofOfMultipleIndexes", zap.Any("readBlockIn", readBlockIn), zap.Any("vmp", vmp))
		nodes, indexes, err := vp.GetMerkleProofOfMultipleIndexes(file, nodesSize, startBlock, endBlock)
		if err != nil {
			return nil, common.NewError("get_merkle_proof_error", err.Error())
		}

		vmp.Nodes = nodes
		vmp.Indexes = indexes
	}

	fileOffset := FMTSize + nodesSize + int64(startBlock)*ChunkSize

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
	var fileObjectPath string
	var err error

	if in.BlockOffset < 0 || in.BlockOffset >= util.FixedMerkleLeaves {
		return nil, common.NewError("invalid_block_number", "Invalid block offset")
	}

	if in.IsPrecommit {
		fileObjectPath = fs.getPreCommitPathForFile(in.AllocationID, in.Hash)
	} else {
		fileObjectPath, err = fs.GetPathForFile(in.AllocationID, in.Hash)
		if err != nil {
			return nil, err
		}
	}

	file, err := os.Open(fileObjectPath)
	if err != nil {
		if in.IsPrecommit {
			fileObjectPath, err = fs.GetPathForFile(in.AllocationID, in.Hash)
			if err != nil {
				return nil, common.NewError("get_file_path_error", err.Error())
			}
			file, err = os.Open(fileObjectPath)
			if err != nil {
				return nil, common.NewError("read_error", err.Error())
			}
		} else {
			return nil, err
		}
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

func (fs *FileStore) getPreCommitDir(allocationID string) string {
	return filepath.Join(fs.getAllocDir(allocationID), PreCommitDir)
}

func (fs *FileStore) getTempPathForFile(allocId, fileName, pathHash, connectionID string) string {
	fileName = sanitizeFileName(fileName)
	return filepath.Join(fs.getAllocTempDir(allocId), fileName+"."+pathHash+"."+connectionID)
}

func (fs *FileStore) getPreCommitPathForFile(allocID, hash string) string {

	return filepath.Join(fs.getPreCommitDir(allocID), getPartialPath(hash, getDirLevelsForFiles()))
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

func (fs *FileStore) pathWriter(path string) io.Reader {

	pathBytes := []byte(path)
	buf := bytes.NewBuffer(pathBytes)
	return buf
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

// Remove any directory traversal characters
func sanitizeFileName(fileName string) string {
	fileName = strings.ReplaceAll(fileName, "../", "")
	fileName = strings.ReplaceAll(fileName, "..\\", "")
	fileName = filepath.Base(fileName)
	return fileName
}
