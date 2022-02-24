package filestore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/errors"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/encryption"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"

	"github.com/0chain/gosdk/constants"
	"github.com/minio/minio-go"
	"golang.org/x/crypto/sha3"

	"github.com/0chain/gosdk/core/util"
)

const (
	OSPathSeperator    string = string(os.PathSeparator)
	ObjectsDirName            = "objects"
	TempObjectsDirName        = "tmp"
	CurrentVersion            = "1.0"
)

type MinioConfiguration struct {
	StorageServiceURL string
	AccessKeyID       string
	SecretAccessKey   string
	BucketName        string
	BucketLocation    string
}

var MinioConfig MinioConfiguration

type IFileBlockGetter interface {
	GetFileBlock(fsStore *FileFSStore, allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error)
}

type FileBlockGetter struct {
}

func (FileBlockGetter) GetFileBlock(fs *FileFSStore, allocationID string, fileData *FileInputData, blockNum, numBlocks int64) ([]byte, error) {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}
	dirPath, destFile := GetFilePathFromHash(fileData.Hash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

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
	RootDirectory   string
	Minio           *minio.Client
	fileBlockGetter IFileBlockGetter
}

type StoreAllocation struct {
	ID              string
	Path            string
	ObjectsPath     string
	TempObjectsPath string
}

var fileFSStore *FileFSStore

func UseDisk() {
	if fileFSStore == nil {
		panic("UseDisk: please SetupFSStore first")
	}

	fileStore = fileFSStore
}

func SetupFSStore(rootDir string) (FileStore, error) {
	if err := createDirs(rootDir); err != nil {
		return nil, err
	}
	return SetupFSStoreI(rootDir, FileBlockGetter{})
}

func SetupFSStoreI(rootDir string, fileBlockGetter IFileBlockGetter) (FileStore, error) {
	fileFSStore = &FileFSStore{
		RootDirectory:   rootDir,
		Minio:           intializeMinio(),
		fileBlockGetter: fileBlockGetter,
	}

	fileStore = fileFSStore

	return fileStore, nil
}

func intializeMinio() *minio.Client {
	if !config.Configuration.MinioStart {
		return nil
	}
	minioClient, err := minio.New(
		MinioConfig.StorageServiceURL,
		MinioConfig.AccessKeyID,
		MinioConfig.SecretAccessKey,
		config.Configuration.MinioUseSSL,
	)
	if err != nil {
		Logger.Panic("Unable to initiaze minio cliet", zap.Error(err))
		panic(err)
	}

	checkBucket(minioClient, MinioConfig.BucketName)
	return minioClient
}

func checkBucket(minioClient *minio.Client, bucketName string) {
	err := minioClient.MakeBucket(bucketName, MinioConfig.BucketLocation)
	if err != nil {
		Logger.Error("Error with make bucket, Will check if bucket exists", zap.Error(err))
		exists, errBucketExists := minioClient.BucketExists(bucketName)
		if errBucketExists == nil && exists {
			Logger.Info("We already own ", zap.Any("bucket_name", bucketName))
		} else {
			Logger.Error("Minio bucket error", zap.Error(errBucketExists), zap.Any("bucket_name", bucketName))
			panic(errBucketExists)
		}
	} else {
		Logger.Info(bucketName + " bucket successfully created")
	}
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
	var size int64
	allocationObj, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return size, err
	}
	err = filepath.Walk(allocationObj.TempObjectsPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func (fs *FileFSStore) GetTotalDiskSizeUsed() (int64, error) {
	var size int64
	err := filepath.Walk(fs.RootDirectory, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func (fs *FileFSStore) GetlDiskSizeUsed(allocationID string) (int64, error) {
	var size int64
	err := filepath.Walk(fs.generateTransactionPath(allocationID), func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

func GetFilePathFromHash(h string) (string, string) {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s", h[0:3])
	for i := 1; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), h[3*i:3*i+3])
	}
	return dir.String(), h[9:]
}

func (fs *FileFSStore) generateTransactionPath(transID string) string {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s", fs.RootDirectory, OSPathSeperator)
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", OSPathSeperator, transID[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", OSPathSeperator, transID[9:])
	return dir.String()
}

func (fs *FileFSStore) SetupAllocation(allocationID string, skipCreate bool) (*StoreAllocation, error) {
	allocation := &StoreAllocation{ID: allocationID}
	allocation.Path = fs.generateTransactionPath(allocationID)
	allocation.ObjectsPath = fmt.Sprintf("%s%s%s", allocation.Path, OSPathSeperator, ObjectsDirName)
	allocation.TempObjectsPath = filepath.Join(allocation.ObjectsPath, TempObjectsDirName)

	if skipCreate {
		return allocation, nil
	}

	//create the allocation object dirs
	err := createDirs(allocation.ObjectsPath)
	if err != nil {
		Logger.Error("allocation_objects_dir_creation_error", zap.Any("allocation_objects_dir_creation_error", err))
		return nil, err
	}

	//create the allocation tmp object dirs
	err = createDirs(allocation.TempObjectsPath)
	if err != nil {
		Logger.Error("allocation_temp_objects_dir_creation_error", zap.Any("allocation_temp_objects_dir_creation_error", err))
		return nil, err
	}

	return allocation, nil
}

func (fs *FileFSStore) GetFileBlockForChallenge(allocationID string, fileData *FileInputData, blockoffset int) (json.RawMessage, util.MerkleTreeI, error) {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return nil, nil, common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}
	dirPath, destFile := GetFilePathFromHash(fileData.Hash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

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
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		Logger.Warn("invalid_allocation", zap.String("allocationID", allocationID), zap.Error(err))
		return nil
	}

	fileObjectPath := fs.generateTempPath(allocation, fileData, connectionID)

	err = os.Remove(fileObjectPath)
	if err != nil {
		Logger.Warn("invalid_path", zap.String("fileObjectPath", fileObjectPath), zap.Error(err))
	}

	return nil
}

func (fs *FileFSStore) generateTempPath(allocation *StoreAllocation, fileData *FileInputData, connectionID string) string {
	return filepath.Join(allocation.TempObjectsPath, fileData.Name+"."+encryption.Hash(fileData.Path)+"."+connectionID)
}

func (fs *FileFSStore) fileCopy(src, dst string) error { //nolint:unused,deadcode // might be used later?
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func (fs *FileFSStore) CommitWrite(allocationID string, fileData *FileInputData, connectionID string) (bool, error) {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return false, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}
	tempFilePath := fs.generateTempPath(allocation, fileData, connectionID)
	//move file from tmp location to the objects folder
	dirPath, destFile := GetFilePathFromHash(fileData.Hash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	err = createDirs(fileObjectPath)
	if err != nil {
		return false, common.NewError("blob_object_dir_creation_error", err.Error())
	}
	fileObjectPath = filepath.Join(fileObjectPath, destFile)
	//if _, err := os.Stat(fileObjectPath); os.IsNotExist(err) {
	err = os.Rename(tempFilePath, fileObjectPath)

	if err != nil {
		return false, common.NewError("blob_object_creation_error", err.Error())
	}
	return true, nil
	//}

	//return false, err
}

func (fs *FileFSStore) DeleteFile(allocationID, contentHash string) error {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}

	dirPath, destFile := GetFilePathFromHash(contentHash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

	if config.Configuration.ColdStorageDeleteCloudCopy {
		err = fs.RemoveFromCloud(contentHash)
		if err != nil {
			Logger.Error("Unable to delete object from minio", zap.Error(err))
		}
	}

	return os.Remove(fileObjectPath)
}

func (fs *FileFSStore) DeleteDir(allocationID, dirPath, connectionID string) error {
	return nil
}

func (fs *FileFSStore) WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	if fileData.IsChunked {
		return fs.WriteChunk(allocationID, fileData, infile, connectionID)
	}

	allocation, err := fs.SetupAllocation(allocationID, false)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}

	tempFilePath := fs.generateTempPath(allocation, fileData, connectionID)
	dest, err := os.Create(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()

	fileRef := &FileOutputData{}

	h := sha256.New()
	bytesBuffer := bytes.NewBuffer(nil)
	multiHashWriter := io.MultiWriter(h, bytesBuffer)
	tReader := io.TeeReader(infile, multiHashWriter)
	merkleHashes := make([]hash.Hash, 1024)
	merkleLeaves := make([]util.Hashable, 1024)
	for idx := range merkleHashes {
		merkleHashes[idx] = sha3.New256()
	}
	fileSize := int64(0)
	for {
		written, err := io.CopyN(dest, tReader, CHUNK_SIZE)

		if err != io.EOF && err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}
		fileSize += written
		dataBytes := bytesBuffer.Bytes()
		merkleChunkSize := 64
		for i := 0; i < len(dataBytes); i += merkleChunkSize {
			end := i + merkleChunkSize
			if end > len(dataBytes) {
				end = len(dataBytes)
			}
			offset := i / merkleChunkSize
			merkleHashes[offset].Write(dataBytes[i:end])
		}

		bytesBuffer.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}
	for idx := range merkleHashes {
		merkleLeaves[idx] = util.NewStringHashable(hex.EncodeToString(merkleHashes[idx].Sum(nil)))
	}

	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)

	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))
	fileRef.Size = fileSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path
	fileRef.MerkleRoot = mt.GetRoot()

	return fileRef, nil
}

// WriteChunk append chunk to temp file
func (fs *FileFSStore) WriteChunk(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	allocation, err := fs.SetupAllocation(allocationID, false)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}

	tempFilePath := fs.generateTempPath(allocation, fileData, connectionID)
	dest, err := NewChunkWriter(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()

	fileRef := &FileOutputData{}

	// the chunk has been rewritten. but it is lost when network is broken, and it is not save in db
	if dest.size > fileData.UploadOffset {
		fileRef.ChunkUploaded = true
	}

	h := sha256.New()
	size, err := dest.WriteChunk(context.TODO(), fileData.UploadOffset, io.TeeReader(infile, h))

	if err != nil {
		return nil, errors.ThrowLog(err.Error(), constants.ErrUnableWriteFile)
	}

	fileRef.Size = size
	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))

	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path

	return fileRef, nil
}

func (fs *FileFSStore) IterateObjects(allocationID string, handler FileObjectHandler) error {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}
	return filepath.Walk(allocation.ObjectsPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && !strings.HasPrefix(path, allocation.TempObjectsPath) {
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
