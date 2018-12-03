package filestore

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	. "0chain.net/logging"
	"go.uber.org/zap"

	"0chain.net/common"
	"0chain.net/encryption"

	"0chain.net/util"
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
	util.CreateDirs(rootDir)
	return &FileFSStore{RootDirectory: rootDir}
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

func (fs *FileFSStore) setupAllocation(allocationID string) (*StoreAllocation, error) {
	allocation := &StoreAllocation{ID: allocationID}
	allocation.Path = fs.generateTransactionPath(allocationID)
	allocation.ObjectsPath = fmt.Sprintf("%s%s%s", allocation.Path, OSPathSeperator, ObjectsDirName)
	allocation.TempObjectsPath = filepath.Join(allocation.ObjectsPath, TempObjectsDirName)

	//create the allocation object dirs
	err := util.CreateDirs(allocation.ObjectsPath)
	if err != nil {
		Logger.Info("allocation_objects_dir_creation_error", zap.Any("allocation_objects_dir_creation_error", err))
		return nil, err
	}

	//create the allocation tmp object dirs
	err = util.CreateDirs(allocation.TempObjectsPath)
	if err != nil {
		Logger.Info("allocation_temp_objects_dir_creation_error", zap.Any("allocation_temp_objects_dir_creation_error", err))
		return nil, err
	}

	return allocation, nil
}

func (fs *FileFSStore) WriteFile(allocationID string, fileData *FileInputData, hdr *multipart.FileHeader) (*FileOutputData, error) {
	allocation, err := fs.setupAllocation(allocationID)
	if err != nil {
		return nil, common.NewError("filestore_setup_error", "Error setting the fs store. "+err.Error())
	}
	mfile, _ := hdr.Open()
	fileSize := int64(0)
	switch t := mfile.(type) {
	case *os.File:
		fi, _ := t.Stat()
		fileSize = fi.Size()
	default:
		sr, _ := mfile.Seek(0, 0)
		fileSize = sr
	}
	mfile.Close()
	Logger.Info("File size", zap.Int64("file_size", fileSize))
	h := sha1.New()
	tempFilePath := filepath.Join(allocation.TempObjectsPath, fileData.Name+"."+encryption.Hash(fileData.Path)+"."+encryption.Hash(string(common.Now())))
	dest, err := os.Create(tempFilePath)
	if err != nil {
		return nil, common.NewError("file_creation_error", err.Error())
	}
	defer dest.Close()
	infile, err := hdr.Open()
	if err != nil {
		return nil, common.NewError("file_reading_error", err.Error())
	}
	merkleHash := sha3.New256()
	multiHashWriter := io.MultiWriter(h, merkleHash)
	tReader := io.TeeReader(infile, multiHashWriter)
	merkleLeaves := make([]util.Hashable, 0)
	for true {
		_, err := io.CopyN(dest, tReader, CHUNK_SIZE)
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
	//Logger.Info("Calculated Merkle root", zap.String("merkle_root", mt.GetRoot()), zap.Int("merkle_leaf_count", len(merkleLeaves)))

	fileRef := &FileOutputData{}
	fileRef.ContentHash = hex.EncodeToString(h.Sum(nil))
	fileRef.Size = fileSize
	fileRef.Name = fileData.Name
	fileRef.Path = fileData.Path
	fileRef.MerkleRoot = mt.GetRoot()
	Logger.Info("File ref", zap.Any("file_ref", fileRef))
	//move file from tmp location to the objects folder
	dirPath, destFile := getFilePathFromHash(fileRef.ContentHash)
	fileObjectPath := filepath.Join(allocation.ObjectsPath, dirPath)
	err = util.CreateDirs(fileObjectPath)
	if err != nil {
		return nil, common.NewError("blob_object_dir_creation_error", err.Error())
	}
	fileObjectPath = filepath.Join(fileObjectPath, destFile)
	err = os.Rename(tempFilePath, fileObjectPath)

	if err != nil {
		return nil, common.NewError("blob_object_creation_error", err.Error())
	}
	return fileRef, nil
}
