package blobber

import (
	"encoding/json"
	
	"net/http"

	
	"github.com/gorilla/mux"
	
)

const (
	AllocationTransactionHeader = "X-Allocation-Transaction"
	BlobberTransactionHeader = "X-Blobber-Transaction"
)

var storageHandler StorageHandler



/*SetupHandlers sets up the necessary API end points */
func SetupHandlers(r *mux.Router) {
	r.HandleFunc("/v1/file/upload/{allocation}", UploadHandler)
	storageHandler = GetStorageHandler()
}

/*UploadHandler is the handler to respond to upload requests fro clients*/
func UploadHandler(respW http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	respW.Header().Set("Content-Type", "application/json")
	
	response := storageHandler.WriteFile(r, vars["allocation"])
	
	if response.Error != nil {
		respW.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(respW).Encode(response)
	return
}
