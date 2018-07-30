package blobber

import (
	"fmt"
	"net/http"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers() {
	http.HandleFunc("/v1/file/upload", UploadHandler)
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(respW http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(respW, "This is the handler for file upload.", r.URL.Path[1:])
}
