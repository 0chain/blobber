package blobber

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/file/upload/{allocation}", UploadHandler)
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	respW.Header().Set("Content-Type", "application/json")

	//io.WriteString(respW, `{"allocation_id": `+vars["allocation"]+`}`)
	n, err := StoreFileFromHTTPRequest(r, vars["allocation"])
	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}

	io.WriteString(respW, `{"num_bytes": `+string(n)+`}`)
	return
}
