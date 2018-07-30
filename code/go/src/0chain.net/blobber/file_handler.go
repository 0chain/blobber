package blobber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

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
		return -1, common.NewError("1001", "Invalid method used for the upload URL. Use multi-part form POST instead")
	}

	file, handler, err := r.FormFile("uploadFile")
	if err != nil {
		fmt.Println(err)
		return 0, common.NewError("1002", err.Error())
	}
	defer file.Close()

	uploadDirPath := r.FormValue("uploadDirPath")
	fmt.Println(uploadDirPath)
	stringPaths := make([]string, 0)
	stringPaths = append(stringPaths, transID)
	stringPaths = append(stringPaths, uploadDirPath)

	j := strings.LastIndex(handler.Filename, path.Ext(handler.Filename))
	name := handler.Filename[:j]
	stringPaths = append(stringPaths, name)

	dirPath := strings.Join(stringPaths, "/")

	fmt.Println(dirPath)

	err = createDirIfNotExist("./" + dirPath)

	if err != nil {
		fmt.Println(err)
		return -1, common.NewError("1003", err.Error())
	}
	f, err := os.OpenFile("./"+dirPath+"/"+handler.Filename, os.O_WRONLY|os.O_CREATE, 0700)
	if err != nil {
		fmt.Println(err)
		return -1, common.NewError("1003", err.Error())
	}
	defer f.Close()

	n, ferr := io.Copy(f, file)
	if ferr != nil {
		fmt.Println(ferr)
		return -1, common.NewError("1004", ferr.Error())
	}
	return int64(n), nil
}
