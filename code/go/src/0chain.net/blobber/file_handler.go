package blobber

import (
	"io"
	"net/http"
	"os"
	"strings"

	. "0chain.net/logging"
	"go.uber.org/zap"

	"0chain.net/common"
)

func createDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

//StoreFileFromHTTPRequest stores the file into the blobber from the HTTP request
func StoreFileFromHTTPRequest(r *http.Request, transID string) (int64, *common.Error) {
	if r.Method == "GET" {
		return -1, common.NewError("invalid_method", "Invalid method used for the upload URL. Use multi-part form POST instead")
	}

	file, handler, err := r.FormFile("uploadFile")
	if err != nil {
		Logger.Debug("", zap.Any("error", err))
		return 0, common.NewError("file_handler_error", err.Error())
	}
	defer file.Close()

	uploadDirPath := r.FormValue("uploadDirPath")
	uploadDirPath = strings.Trim(uploadDirPath, "/")
	Logger.Info("Upload", zap.Any("Directory Path", uploadDirPath))
	stringPaths := make([]string, 0)
	stringPaths = append(stringPaths, transID)
	stringPaths = append(stringPaths, uploadDirPath)

	dirPath := strings.Join(stringPaths, "/")

	Logger.Debug("DirectoryPath", zap.Any("Path", dirPath))

	err = createDirIfNotExist("./" + dirPath)

	if err != nil {
		Logger.Debug("", zap.Any("error", err))
		return -1, common.NewError("dir_creation_error", err.Error())
	}
	f, err := os.OpenFile("./"+dirPath+"/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		Logger.Debug("", zap.Any("error", err))
		return -1, common.NewError("file_creation_error", err.Error())
	}
	defer f.Close()

	n, ferr := io.Copy(f, file)
	if ferr != nil {
		Logger.Debug("", zap.Any("error", ferr))
		return -1, common.NewError("file_write_error", ferr.Error())
	}
	return int64(n), nil
}
