package blobber

import (
	"fmt"
	"html"
	"net/http"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers() {
	http.HandleFunc("/v1/file/upload", UploadHandler)
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(respW http.ResponseWriter, r *http.Request) {
	
}

