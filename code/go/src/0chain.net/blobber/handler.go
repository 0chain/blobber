package blobber

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "0chain.net/logging"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
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

	n, err := StoreFileFromHTTPRequest(r, vars["allocation"])
	Logger.Info("n", zap.Any("n", n))
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
