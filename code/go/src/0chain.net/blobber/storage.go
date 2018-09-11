package blobber

import (
	"net/http"

	"0chain.net/common"
)

type UploadResult struct {
	Filename string `json:"filename"`
	Size int64 `json:"size,omitempty"`
	Hash string `json:"content_hash,omitempty"`
	Error *common.Error `json:"error,omitempty"`
}

//UploadResponse - response to upload or write requests
type UploadResponse struct {
	Result []UploadResult `json:"result"`
	Error *common.Error `json:"error,omitempty"`
}

type DownloadResponse struct {
	Filename string
	Size string
	ContentType string
	Path string
}

type MetaInfo struct {
	Filename string `json:"filename"`
	CustomMeta string `json:"custom_meta"`
	Size int64 `json:"size"`
	ContentHash string `json:"content_hash"`
}

type FileMeta struct {
	ID string `json:"id"`
	Meta []MetaInfo `json:"meta"`
}

//StorageHandler - interfact for handling storage requests
type StorageHandler interface {
	WriteFile(r *http.Request, allocationID string) (UploadResponse)
	DownloadFile(r *http.Request, allocationID string) (*DownloadResponse, *common.Error)
	GetFileMeta(r *http.Request, allocationID string) (*FileMeta, *common.Error)
}

//SHandler - Singleton for the storage handler
var SHandler StorageHandler

/*GetStorageHandler - get the storage handler that is setup */
func GetStorageHandler() StorageHandler {
	return SHandler
}

func  GenerateUploadResponseWithError(err *common.Error) UploadResponse{
	var response UploadResponse
	response.Error = err
	return response
}
