package blobber

import (
	
	"net/http"
	"os"
	"io"
	
	"fmt"
	"bytes"

	. "0chain.net/logging"
	"go.uber.org/zap"

	"0chain.net/common"
	"0chain.net/encryption"
	"crypto/sha1"
	"encoding/hex"
)

//ObjectStorageHandler - implments the StorageHandler interface
type ObjectStorageHandler struct {
	RootDirectory string
}

/*SetupFSStorageHandler - Setup a file system based block storage */
func SetupObjectStorageHandler(rootDir string) {
	createDirIfNotExist(rootDir)
	SHandler = &ObjectStorageHandler{RootDirectory: rootDir}
}

func createDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

type EntryType string

const (
	FILE EntryType = "f"
	DIRECTORY EntryType = "d"
)

type StoreObject struct {
	Name string
	PathHash string
	Path string
}

type DirectoryTreeEntry struct {
	entryType EntryType
	StoreObject
}

type DirectoryTree struct {
	entries []DirectoryTreeEntry
	StoreObject
}

// type File struct {
// 	*StoreObject
// }


func (dt *StoreObject) GetHashFromPath() string {
	return encryption.Hash(dt.Path)
}

func (fsh *ObjectStorageHandler) loadDirectoryTree(dirPath string) *DirectoryTree {
	dt:= &DirectoryTree{}
	dt.Path = dirPath
	dt.PathHash = dt.GetHashFromPath()
	dt.Name = dirPath
	return dt
} 


func (fsh *ObjectStorageHandler) generateTransactionPath(transID string) string{

	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s", fsh.RootDirectory, string(os.PathSeparator))
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), transID[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", string(os.PathSeparator), transID[9:])
	return dir.String()
}


//WriteFile stores the file into the blobber files system from the HTTP request
func (fsh *ObjectStorageHandler) WriteFile(r *http.Request, transID string) (int64, *common.Error) {
	if r.Method == "GET" {
		return -1, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead")
	}

	err := createDirIfNotExist(fsh.generateTransactionPath(transID))
	if err != nil {
		Logger.Info("", zap.Any("error", err))
		return -1, common.NewError("allocation_dir_creation_error", err.Error())
	}


	//get the multipart reader for the request.
	reader, err := r.MultipartReader()

	if err != nil {
		Logger.Info("", zap.Any("error", err))
		return 0, common.NewError("file_handler_error", err.Error())
	}

	//copy each part to destination.
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}

		//if part.FileName() is empty, skip this iteration.
		if part.FileName() == "" {
			continue
		}

		h := sha1.New()
	    // dest, err := os.Create("tmpfile.txt")
	    // if err != nil {
	    //     return -1, common.NewError("file_creation_error", err.Error())
	    // }
	    // defer dest.Close()
	    //t := io.TeeReader(part, h)
	    _, err = io.Copy(h, part)
	    if err != nil {
	        return -1, common.NewError("file_write_error", err.Error())
	    }

	    //return h.Sum(nil), nil
		Logger.Info("", zap.Any("hash", hex.EncodeToString(h.Sum(nil))))
	}


	// file, handler, err := r.FormFile("uploadFile")
	// if err != nil {
	// 	Logger.Info("", zap.Any("error", err))
	// 	return 0, common.NewError("file_handler_error", err.Error())
	// }
	// defer file.Close()

	

	// root_dt := fsh.loadDirectoryTree(transID + os.PathSeparator)

	// Logger.Info("FileHash", zap.Any("FileHash", root_dt.PathHash))

	// Logger.Info("FileName", zap.Any("FileName", handler.Filename))
	return 100, nil



	// uploadDirPath := r.FormValue("uploadDirPath")





	// uploadDirPath = strings.Trim(uploadDirPath, os.PathSeparator)
	// Logger.Info("Upload", zap.Any("Directory Path", uploadDirPath))
	// stringPaths := make([]string, 0)
	// stringPaths = append(stringPaths, transID)
	// stringPaths = append(stringPaths, uploadDirPath)

	// dirPath := strings.Join(stringPaths, os.PathSeparator)

	// Logger.Debug("DirectoryPath", zap.Any("Path", dirPath))

	// err = createDirIfNotExist("./" + dirPath)

	// if err != nil {
	// 	Logger.Debug("", zap.Any("error", err))
	// 	return -1, common.NewError("dir_creation_error", err.Error())
	// }
	// f, err := os.OpenFile("./"+dirPath+"/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0700)
	// if err != nil {
	// 	Logger.Debug("", zap.Any("error", err))
	// 	return -1, common.NewError("file_creation_error", err.Error())
	// }
	// defer f.Close()

	// n, ferr := io.Copy(f, file)
	// if ferr != nil {
	// 	Logger.Debug("", zap.Any("error", ferr))
	// 	return -1, common.NewError("file_write_error", ferr.Error())
	// }
	// return int64(n), nil
}

