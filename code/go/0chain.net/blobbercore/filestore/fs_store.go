package filestore

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	. "0chain.net/core/logging"
	"go.uber.org/zap"

	"0chain.net/core/common"
	"0chain.net/core/encryption"

	"0chain.net/blobbercore/config"

	"0chain.net/core/util"
	"github.com/minio/minio-go"
	"golang.org/x/crypto/sha3"
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

func (FileBlockGetter) GetFileBlock(fs *FileFSStore, allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error) {
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

	filesize := int(fileinfo.Size())
	maxBlockNum := int64(filesize / CHUNK_SIZE)
	// check for any left over bytes. Add one more go routine if required.
	if remainder := filesize % CHUNK_SIZE; remainder != 0 {
		maxBlockNum++
	}

	if blockNum > maxBlockNum || blockNum < 1 {
		return nil, common.NewError("invalid_block_number", "Invalid block number")
	}
	buffer := make([]byte, CHUNK_SIZE*numBlocks)
	n, err := file.ReadAt(buffer, ((blockNum - 1) * CHUNK_SIZE))
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

func SetupFSStore(rootDir string) (FileStore, error) {
	if err := createDirs(rootDir); err != nil {
		return nil, err
	}
	return SetupFSStoreI(rootDir, FileBlockGetter{})
}

func SetupFSStoreI(rootDir string, fileBlockGetter IFileBlockGetter) (FileStore, error) {
	fsStore = &FileFSStore{
		RootDirectory:   rootDir,
		Minio:           intializeMinio(),
		fileBlockGetter: fileBlockGetter,
	}
	return fsStore, nil
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

func GetFilePathFromHash(hash string) (string, string) {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s", hash[0:3])
	for i := 1; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), hash[3*i:3*i+3])
	}
	return dir.String(), hash[9:]
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

	merkleHashes := make([]hash.Hash, 1024)
	merkleLeaves := make([]util.Hashable, 1024)
	for idx := range merkleHashes {
		merkleHashes[idx] = sha3.New256()
	}
	bytesBuf := bytes.NewBuffer(make([]byte, 0))
	for {
		_, err := io.CopyN(bytesBuf, file, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			return nil, nil, common.NewError("file_write_error", err.Error())
		}
		dataBytes := bytesBuf.Bytes()
		tmpBytes := make([]byte, len(dataBytes))
		copy(tmpBytes, dataBytes)
		merkleChunkSize := 64
		for i := 0; i < len(tmpBytes); i += merkleChunkSize {
			end := i + merkleChunkSize
			if end > len(tmpBytes) {
				end = len(tmpBytes)
			}
			offset := i / merkleChunkSize
			merkleHashes[offset].Write(tmpBytes[i:end])
			if offset == blockoffset {
				returnBytes = append(returnBytes, tmpBytes[i:end]...)
			}
		}
		bytesBuf.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}

	for idx := range merkleHashes {
		merkleLeaves[idx] = util.NewStringHashable(hex.EncodeToString(merkleHashes[idx].Sum(nil)))
	}
	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)

	return returnBytes, mt, nil
}

func (fs *FileFSStore) GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64, numBlocks int64) ([]byte, error) {
	return fs.fileBlockGetter.GetFileBlock(fs, allocationID, fileData, blockNum, numBlocks)
}

func (fs *FileFSStore) DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}

	fileObjectPath := fs.generateTempPath(allocation, fileData, connectionID)

	return os.Remove(fileObjectPath)
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

func (fs *FileFSStore) DeleteFile(allocationID string, contentHash string) error {
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

func (fs *FileFSStore) GetMerkleTreeForFile(allocationID string, fileData *FileInputData) (util.MerkleTreeI, error) {
	allocation, err := fs.SetupAllocation(allocationID, true)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
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
	//merkleHash := sha3.New256()
	tReader := file //io.TeeReader(file, merkleHash)
	//merkleLeaves := make([]util.Hashable, 0)
	merkleHashes := make([]hash.Hash, 1024)
	merkleLeaves := make([]util.Hashable, 1024)
	for idx := range merkleHashes {
		merkleHashes[idx] = sha3.New256()
	}
	bytesBuf := bytes.NewBuffer(make([]byte, 0))
	for {
		_, err := io.CopyN(bytesBuf, tReader, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}
		dataBytes := bytesBuf.Bytes()
		merkleChunkSize := 64
		for i := 0; i < len(dataBytes); i += merkleChunkSize {
			end := i + merkleChunkSize
			if end > len(dataBytes) {
				end = len(dataBytes)
			}
			offset := i / merkleChunkSize
			merkleHashes[offset].Write(dataBytes[i:end])
		}
		//merkleLeaves = append(merkleLeaves, util.NewStringHashable(hex.EncodeToString(merkleHash.Sum(nil))))
		//merkleHash.Reset()
		bytesBuf.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}
	for idx := range merkleHashes {
		merkleLeaves[idx] = util.NewStringHashable(hex.EncodeToString(merkleHashes[idx].Sum(nil)))
	}

	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)

	return mt, nil
}

func (fs *FileFSStore) WriteFile(allocationID string, fileData *FileInputData,
	infile multipart.File, connectionID string) (*FileOutputData, error) {

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
	var fileReader io.Reader = infile

	if fileData.IsResumable {
		h := sha1.New()
		offset, err := dest.WriteChunk(context.TODO(), fileData.UploadOffset, io.TeeReader(fileReader, h))

		if err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}

		fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))
		fileRef.Size = dest.Size()
		fileRef.Name = fileData.Name
		fileRef.Path = fileData.Path
		fileRef.UploadOffset = fileData.UploadOffset + offset
		fileRef.UploadLength = fileData.UploadLength

		if !fileData.IsFinal {
			//skip to compute hash until the last chunk is uploaded
			return fileRef, nil
		}

		fileReader = dest
	}

	h := sha1.New()
	bytesBuffer := bytes.NewBuffer(nil)
	multiHashWriter := io.MultiWriter(h, bytesBuffer)
	tReader := io.TeeReader(fileReader, multiHashWriter)
	merkleHashes := make([]hash.Hash, 1024)
	merkleLeaves := make([]util.Hashable, 1024)
	for idx := range merkleHashes {
		merkleHashes[idx] = sha3.New256()
	}
	fileSize := int64(0)
	for {
		var written int64

		if fileData.IsResumable {
			//all chunks have been written, only read bytes from local file , and compute hash
			written, err = io.CopyN(ioutil.Discard, tReader, CHUNK_SIZE)
		} else {
			written, err = io.CopyN(dest, tReader, CHUNK_SIZE)
		}

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

	//only update hash for whole file when it is not a resumable upload or is final chunk.
	if !fileData.IsResumable || fileData.IsFinal {
		fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))
	}

	fileRef.Size = fileSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path
	fileRef.MerkleRoot = mt.GetRoot()
	fileRef.UploadOffset = fileSize
	fileRef.UploadLength = fileData.UploadLength

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
			h := sha1.New()
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
	if _, err := fs.Minio.StatObject(MinioConfig.BucketName, fileHash, minio.StatObjectOptions{}); err == nil {
		return fs.Minio.RemoveObject(MinioConfig.BucketName, fileHash)
	}
	return nil
}
