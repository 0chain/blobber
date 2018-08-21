package blobber

import (
	"net/http"

	"0chain.net/common"
)

//StorageHandler - interfact for handling storage requests
type StorageHandler interface {
	WriteFile(r *http.Request, transID string) (int64, *common.Error)
}

//SHandler - Singleton for the storage handler
var SHandler StorageHandler

/*GetStorageHandler - get the storage handler that is setup */
func GetStorageHandler() StorageHandler {
	return SHandler
}
