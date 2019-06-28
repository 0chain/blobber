package filestore

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	. "0chain.net/core/logging"
	"go.uber.org/zap"

	"0chain.net/core/common"
	"0chain.net/core/encryption"

	"0chain.net/core/util"
	"golang.org/x/crypto/sha3"
)

const (
	OSPathSeperator    string = string(os.PathSeparator)
	ObjectsDirName            = "objects"
	TempObjectsDirName        = "tmp"
	CurrentVersion            = "1.0"
)

type FileFSStore struct {
	RootDirectory string
}

type StoreAllocation struct {
	ID              string
	Path            string
	ObjectsPath     string
	TempObjectsPath string
}

func SetupFSStore(rootDir string) FileStore {
	createDirs(rootDir)
	fsStore = &FileFSStore{RootDirectory: rootDir}
	return fsStore
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
	allocationObj, err := fs.setupAllocation(allocationID, true)
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

func getFilePathFromHash(hash string) (string, string) {
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

func (fs *FileFSStore) setupAllocation(allocationID string, skipCreate bool) (*StoreAllocation, error) {
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

func (fs *FileFSStore) GetFileBlock(allocationID string, fileData *FileInputData, blockNum int64) (json.RawMessage, error) {
	allocation, err := fs.setupAllocation(allocationID, true)
	if err != nil {
		return nil, common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}
	dirPath, destFile := getFilePathFromHash(fileData.Hash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

	file, err := os.Open(fileObjectPath)
	if err != nil {
		return nil, err
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
	buffer := make([]byte, CHUNK_SIZE)
	n, err := file.ReadAt(buffer, ((blockNum - 1) * CHUNK_SIZE))
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buffer[:n], nil
}

func (fs *FileFSStore) DeleteTempFile(allocationID string, fileData *FileInputData, connectionID string) error {
	allocation, err := fs.setupAllocation(allocationID, true)
	if err != nil {
		return common.NewError("invalid_allocation", "Invalid allocation. "+err.Error())
	}

	fileObjectPath := fs.generateTempPath(allocation, fileData, connectionID)

	return os.Remove(fileObjectPath)
}

func (fs *FileFSStore) generateTempPath(allocation *StoreAllocation, fileData *FileInputData, connectionID string) string {
	return filepath.Join(allocation.TempObjectsPath, fileData.Name+"."+encryption.Hash(fileData.Path)+"."+connectionID)
}

func (fs *FileFSStore) fileCopy(src, dst string) error {
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
	allocation, err := fs.setupAllocation(allocationID, true)
	if err != nil {
		return false, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}
	tempFilePath := fs.generateTempPath(allocation, fileData, connectionID)
	//move file from tmp location to the objects folder
	dirPath, destFile := getFilePathFromHash(fileData.Hash)
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
	allocation, err := fs.setupAllocation(allocationID, true)
	if err != nil {
		return common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}

	dirPath, destFile := getFilePathFromHash(contentHash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

	return os.Remove(fileObjectPath)
}

func (fs *FileFSStore) GetMerkleTreeForFile(allocationID string, fileData *FileInputData) (util.MerkleTreeI, error) {
	allocation, err := fs.setupAllocation(allocationID, true)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}
	dirPath, destFile := getFilePathFromHash(fileData.Hash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	fileObjectPath = filepath.Join(fileObjectPath, destFile)

	file, err := os.Open(fileObjectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	merkleHash := sha3.New256()
	tReader := io.TeeReader(file, merkleHash)
	merkleLeaves := make([]util.Hashable, 0)
	bytesBuf := bytes.NewBuffer(make([]byte, 0))
	for true {
		_, err := io.CopyN(bytesBuf, tReader, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}
		merkleLeaves = append(merkleLeaves, util.NewStringHashable(hex.EncodeToString(merkleHash.Sum(nil))))
		merkleHash.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}

	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)

	return mt, nil
}

func (fs *FileFSStore) WriteFile(allocationID string, fileData *FileInputData, infile multipart.File, connectionID string) (*FileOutputData, error) {
	allocation, err := fs.setupAllocation(allocationID, false)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}

	h := sha1.New()
	tempFilePath := fs.generateTempPath(allocation, fileData, connectionID)
	dest, err := os.Create(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()
	// infile, err := hdr.Open()
	// if err != nil {
	// 	return nil, common.NewError("file_reading_error", err.Error())
	// }
	merkleHash := sha3.New256()
	multiHashWriter := io.MultiWriter(h, merkleHash)
	tReader := io.TeeReader(infile, multiHashWriter)
	merkleLeaves := make([]util.Hashable, 0)
	fileSize := int64(0)
	for true {
		written, err := io.CopyN(dest, tReader, CHUNK_SIZE)
		if err != io.EOF && err != nil {
			return nil, common.NewError("file_write_error", err.Error())
		}
		fileSize += written
		merkleLeaves = append(merkleLeaves, util.NewStringHashable(hex.EncodeToString(merkleHash.Sum(nil))))
		merkleHash.Reset()
		if err != nil && err == io.EOF {
			break
		}
	}
	//Logger.Info("File size", zap.Int64("file_size", fileSize))
	var mt util.MerkleTreeI = &util.MerkleTree{}
	mt.ComputeTree(merkleLeaves)
	//Logger.Info("Calculated Merkle root", zap.String("merkle_root", mt.GetRoot()), zap.Int("merkle_leaf_count", len(merkleLeaves)))

	fileRef := &FileOutputData{}
	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))
	fileRef.Size = fileSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path
	fileRef.MerkleRoot = mt.GetRoot()

	return fileRef, nil
}
