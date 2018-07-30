package blobber

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

type UploadResponse struct {
	NumBytes int64 `json:"num_bytes"`
}

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
	fmt.Println(n)
	if err != nil {
		respW.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(respW).Encode(err)
		return
	}
	c := UploadResponse{NumBytes: n}
	json.NewEncoder(respW).Encode(c)
	return
}
