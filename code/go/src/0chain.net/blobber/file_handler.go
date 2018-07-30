package blobber

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"0chain.net/common"
)

//StoreFileFromHTTPRequest stores the file into the blobber from the HTTP request
func StoreFileFromHTTPRequest(r *http.Request, transID string) (int, *common.Error) {
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
	stringPaths := make([]string, 3)
	stringPaths = append(stringPaths, transID)
	stringPaths = append(stringPaths, uploadDirPath)
	stringPaths = append(stringPaths, handler.Filename)

	dirPath := strings.Join(stringPaths, "/")

	f, err := os.OpenFile("./"+dirPath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Println(err)
		return 0, common.NewError("1003", err.Error())
	}
	defer f.Close()

	n, ferr := io.Copy(f, file)

	return int(n), common.NewError("1004", ferr.Error())
}
