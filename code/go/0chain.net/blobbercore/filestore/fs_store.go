package filestore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/errors"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/gosdk/constants"
	"github.com/minio/minio-go"

	"github.com/0chain/gosdk/core/util"
)

type MinioConfiguration struct {
	StorageServiceURL string
	AccessKeyID       string
	SecretAccessKey   string
	BucketName        string
	BucketLocation    string
}

var MinioConfig MinioConfiguration

var contentHashMapLock = common.GetLocker()

func getKey(allocID, contentHash string) string {
	return encryption.Hash(allocID + contentHash)
}

type IFileBlockGetter interface {
	GetFileBlock(fsStore *FileFSStore, allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)
}

type FileBlockGetter struct {
}

func (FileBlockGetter) GetFileBlock(fs *FileFSStore, allocationID string, fileData *FileInputData, blockNum, numBlocks int64) ([]byte, error) {
	fileObjectPath, err := GetPathForFile(allocationID, fileData.Hash)
	if err != nil {
		return nil, common.NewError("get_file_path_error", err.Error())
	}
	file, err := os.Open(fileObjectPath)
	if err != nil {
		if os.IsNotExist(err) && fileData.OnCloud {
			err = fs.DownloadFromCloud(fileData.Hash, fileObjectPath)
			if err != nil {
				return nil, common.NewError("minio_download_failed", "Unable to download from minio with err "+err.Error())
			}
			file, err = os.Open(fileObjectPath)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	defer file.Close()
	fileinfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	filesize := fileinfo.Size()
	maxBlockNum := filesize / fileData.ChunkSize
	// check for any left over bytes. Add one more go routine if required.
	if remainder := filesize % fileData.ChunkSize; remainder != 0 {
		maxBlockNum++
	}

	if blockNum > maxBlockNum || blockNum < 1 {
		return nil, common.NewError("invalid_block_number", "Invalid block number")
	}
	buffer := make([]byte, fileData.ChunkSize*numBlocks)
	n, err := file.ReadAt(buffer, ((blockNum - 1) * fileData.ChunkSize))
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buffer[:n], nil
}

type FileFSStore struct {
	Minio           *minio.Client
	fileBlockGetter IFileBlockGetter
}

var fileFSStore *FileFSStore

func SetupFSStore(mp string) error {

	_, err := SetupFSStoreI(mp, FileBlockGetter{})
	return err
}

func SetupFSStoreI(mp string, fileBlockGetter IFileBlockGetter) (FileStore, error) {

	err := initManager(mp)
	if err != nil {
		return nil, err
	}

	minioClient, err := intializeMinio()
	if err != nil {
		return nil, err
	}

	fileFSStore = &FileFSStore{
		Minio:           minioClient,
		fileBlockGetter: fileBlockGetter,
	}

	fileStore = fileFSStore

	return fileStore, nil
}

func intializeMinio() (*minio.Client, error) {
	if !config.Configuration.MinioStart {
		return nil, nil
	}
	minioClient, err := minio.New(
		MinioConfig.StorageServiceURL,
		MinioConfig.AccessKeyID,
		MinioConfig.SecretAccessKey,
		config.Configuration.MinioUseSSL,
	)
	if err != nil {
		return nil, errors.New("minio_initialize_error", err.Error())
	}

	if err := checkBucket(minioClient, MinioConfig.BucketName); err != nil {
		return nil, err
	}
	return minioClient, nil
}

func checkBucket(minioClient *minio.Client, bucketName string) error {
	err := minioClient.MakeBucket(bucketName, MinioConfig.BucketLocation)
	if err != nil {
		logging.Logger.Error("Error with make bucket, checking if bucket exists", zap.Error(err))
		exists, errBucketExists := minioClient.BucketExists(bucketName)
		if errBucketExists == nil && exists {
			logging.Logger.Info("Bucket exists already", zap.Any("bucket_name", bucketName))
		} else {
			logging.Logger.Error("Minio bucket error", zap.Error(errBucketExists), zap.Any("bucket_name", bucketName))
			return errBucketExists
		}
	} else {
		logging.Logger.Info(bucketName + " bucket successfully created")
	}

	return nil
}

func createDirs(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

func (fs *FileFSStore) GetTempPathSize(allocationID string) (int64, error) {
	return int64(getTempFilesSize(allocationID)), nil
}

func (fs *FileFSStore) GetTotalDiskSizeUsed() (int64, error) {
	return int64(getDiskUsedByAllocations()), nil

}

func (fs *FileFSStore) GetlDiskSizeUsed(allocationID string) (int64, error) {
	return int64(getAllocationSpaceUsed(allocationID)), nil
}

func (fs *FileFSStore) GetFileBlockForChallenge(allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error) {
	fileObjectPath, err := GetPathForFile(allocationID, fileData.Hash)
	if err != nil {
		return nil, nil, common.NewError("get_file_path_error", err.Error())
	}

	file, err := os.Open(fileObjectPath)
	if err != nil {
		if os.IsNotExist(err) && fileData.OnCloud {
			err = fs.DownloadFromCloud(fileData.Hash, fileObjectPath)
			if err != nil {
				return nil, nil, common.NewError("minio_download_failed", "Unable to download from minio with err "+err.Error())
			}
			file, err = os.Open(fileObjectPath)
			if err != nil {
				return nil, nil, err
			}
		} else {
			return nil, nil, err
		}
	}
	defer file.Close()

	if blockoffset < 0 || blockoffset >= 1024 {
		return nil, nil, common.NewError("invalid_block_number", "Invalid block offset")
	}

	var returnBytes []byte

	fi, _ := file.Stat()

	numChunks := int(math.Ceil(float64(fi.Size()) / float64(fileData.ChunkSize)))

	fmt := util.NewFixedMerkleTree(int(fileData.ChunkSize))

	bytesBuf := bytes.NewBuffer(make([]byte, 0))
	for chunkIndex := 0; chunkIndex < numChunks; chunkIndex++ {
		written, err := io.CopyN(bytesBuf, file, fileData.ChunkSize)

		if written > 0 {
			dataBytes := bytesBuf.Bytes()

			err2 := fmt.Write(dataBytes, chunkIndex)
			if err2 != nil {
				return nil, nil, errors.ThrowLog(err2.Error(), constants.ErrUnableHash)
			}

			merkleChunkSize := int(fileData.ChunkSize) / 1024

			if merkleChunkSize == 0 {
				merkleChunkSize = 1
			}

			offset := 0

			for i := 0; i < len(dataBytes); i += merkleChunkSize {
				end := i + merkleChunkSize
				if end > len(dataBytes) {
					end = len(dataBytes)
				}

				if offset == blockoffset {
					returnBytes = append(returnBytes, dataBytes[i:end]...)
				}

				offset++
				if offset >= 1024 {
					offset = 1
				}
			}
			bytesBuf.Reset()
		}

		if err != nil && err == io.EOF {
			break
		}
	}

	return returnBytes, fmt.GetMerkleTree(), nil
}

func (fs *FileFSStore) GetFileBlock(allocationID string, fileData *FileInputData, blockNum, numBlocks int64) ([]byte, error) {
	return fs.fileBlockGetter.GetFileBlock(fs, allocationID, fileData, blockNum, numBlocks)
}

func (fs *FileFSStore) DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error {
	fileObjectPath := getTempPathForFile(allocationID, fileData.Name, encryption.Hash(fileData.Path), connectionID)

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

	updateAllocTempFileSize(allocationID, -size)

	return nil
}

func (fs *FileFSStore) CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error) {
	key := getKey(allocationID, fileData.Hash)
	l, _ := contentHashMapLock.GetLock(key)
	l.Lock()

	tempFilePath := getTempPathForFile(allocationID, fileData.Name, encryption.Hash(fileData.Path), connectionID)
	finfo, err := os.Stat(tempFilePath)
	if err != nil {
		return false, common.NewError("stat_error", err.Error())
	}
	fileSize := finfo.Size()
	fileObjectPath, err := GetPathForFile(allocationID, fileData.Hash)
	if err != nil {
		return false, common.NewError("get_file_path_error", err.Error())
	}
	err = createDirs(filepath.Dir(fileObjectPath))
	if err != nil {
		return false, common.NewError("blob_object_dir_creation_error", err.Error())
	}

	//move file from tmp location to the objects folder
	err = os.Rename(tempFilePath, fileObjectPath)
	l.Unlock()

	if err != nil {
		return false, common.NewError("blob_object_creation_error", err.Error())
	}

	updateAllocTempFileSize(allocationID, -fileSize)
	// Each commit write should add 1 to file number because of the following:
	// 1. NewFile: Obvioulsy needs to increment by 1
	// 2. UpdateFile: First it will delete, decrements file number by 1 and will Call CommitWrite
	// 3. Rename: Doesn't call CommitWrite i.e. doesn't do anything with file data
	// 4. Copy: Doesn't call CommitWrite. Same as Rename
	// 5. Move: It is Copy + Delete. Delete will not delete file if ref exists in database. i.e. copy would create
	// ref that refers to this file therefore it will be skipped
	incrDecrAllocFileSizeAndNumber(allocationID, fileSize, 1)
	return true, nil
}

func (fs *FileFSStore) DeleteFile(allocationID, contentHash string) error {
	fileObjectPath, err := GetPathForFile(allocationID, contentHash)
	if err != nil {
		return common.NewError("get_file_path_error", err.Error())
	}

	finfo, err := os.Stat(fileObjectPath)
	if err != nil {
		return err
	}
	size := finfo.Size()

	key := getKey(allocationID, contentHash)

	// isNew is checked if a fresh lock is acquired. If lock is just holded by this process then it will actually delete
	// the file.
	// If isNew is false, either same content is being written or deleted. Therefore, this process can rely on other process
	// to either keep or delete file
	l, isNew := contentHashMapLock.GetLock(key)
	if !isNew {
		incrDecrAllocFileSizeAndNumber(allocationID, -size, -1)

		return errors.New("not_new_lock",
			fmt.Sprintf("lock is acquired by other process to process on content. allocation id: %s content hash: %s",
				allocationID, contentHash))
	}
	l.Lock()

	if config.Configuration.ColdStorageDeleteCloudCopy {
		err = fs.RemoveFromCloud(contentHash)
		if err != nil {
			logging.Logger.Error("Unable to delete object from minio", zap.Error(err))
		}
	}

	err = os.Remove(fileObjectPath)
	if err != nil {
		return err
	}

	incrDecrAllocFileSizeAndNumber(allocationID, -size, -1)
	return nil
}

func (fs *FileFSStore) DeleteDir(allocationID, dirPath, connectionID string) error {
	return nil
}

func (fs *FileFSStore) WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	tempFilePath := getTempPathForFile(allocationID, fileData.Name, encryption.Hash(fileData.Path), connectionID)
	if err := createDirs(filepath.Dir(tempFilePath)); err != nil {
		return nil, common.NewError("dir_creation_error", err.Error())
	}

	dest, err := NewChunkWriter(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()

	fileRef := &FileOutputData{}
	/* Todos
	   cloud object inconsistency due to same content hash
	*/

	// the chunk has been rewritten. but network was broken, and it is not save in db
	if dest.size > fileData.UploadOffset {
		fileRef.ChunkUploaded = true
	}

	h := sha256.New()
	size, err := dest.WriteChunk(context.TODO(), fileData.UploadOffset, io.TeeReader(infile, h))

	if err != nil {
		return nil, errors.ThrowLog(err.Error(), constants.ErrUnableWriteFile)
	}

	updateAllocTempFileSize(allocationID, size)

	fileRef.Size = size
	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))

	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path

	return fileRef, nil
}

func (fs *FileFSStore) IterateObjects(allocationID string, handler FileObjectHandler) error {
	allocDir := getAllocDir(allocationID)
	tmpPrefix := filepath.Join(allocDir, TempDir)
	return filepath.Walk(allocDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && !strings.HasPrefix(path, tmpPrefix) {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return nil
			}
			handler(hex.EncodeToString(h.Sum(nil)), info.Size())
		}
		return nil
	})
}

func (fs *FileFSStore) UploadToCloud(fileHash, filePath string) error {
	_, err := fs.Minio.FPutObject(MinioConfig.BucketName, fileHash, filePath, minio.PutObjectOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (fs *FileFSStore) DownloadFromCloud(fileHash, filePath string) error {
	return fs.Minio.FGetObject(MinioConfig.BucketName, fileHash, filePath, minio.GetObjectOptions{})
}

func (fs *FileFSStore) RemoveFromCloud(fileHash string) error {
	if fs != nil && fs.Minio != nil {
		_, err := fs.Minio.StatObject(MinioConfig.BucketName, fileHash, minio.StatObjectOptions{})
		if err == nil {
			return fs.Minio.RemoveObject(MinioConfig.BucketName, fileHash)
		}
	}
	return nil
}
