package blobber

import (
	"context"
	"net/http"

	"0chain.net/allocation"
	"0chain.net/datastore"
	"0chain.net/filestore"
	"0chain.net/readmarker"
	"0chain.net/reference"
	"0chain.net/writemarker"
)

type UploadResult struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Hash       string `json:"content_hash"`
	MerkleRoot string `json:"merkle_root"`
}

type CommitResult struct {
	AllocationRoot string                         `json:"allocation_root"`
	WriteMarker    *writemarker.WriteMarker       `json:"write_marker"`
	Success        bool                           `json:"success"`
	ErrorMessage   string                         `json:"error_msg,omitempty"`
	Changes        []*allocation.AllocationChange `json:"-"`
	//Result         []*UploadResult         `json:"result"`
}

type ListResult struct {
	AllocationRoot string                   `json:"allocation_root"`
	Meta           map[string]interface{}   `json:"meta_data"`
	Entities       []map[string]interface{} `json:"list"`
}

type ObjectPathResult struct {
	*reference.ObjectPath
	AllocationRoot string `json:"allocation_root"`
}

type DownloadResponse struct {
	Success      bool                   `json:"success"`
	Data         []byte                 `json:"data"`
	AllocationID string                 `json:"-"`
	Path         string                 `json:"-"`
	LatestRM     *readmarker.ReadMarker `json:"latest_rm"`
}

type ReferencePathResult struct {
	*reference.ReferencePath
	LatestWM *writemarker.WriteMarker `json:"latest_write_marker"`
}

//StorageHandler - interfact for handling storage requests
type StorageHandler interface {
	WriteFile(ctx context.Context, r *http.Request) (*UploadResult, error)
	CommitWrite(ctx context.Context, r *http.Request) (*CommitResult, error)
	DownloadFile(ctx context.Context, r *http.Request) (*DownloadResponse, error)
	GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error)
	GetFileStats(ctx context.Context, r *http.Request) (interface{}, error)
	ListEntities(ctx context.Context, r *http.Request) (*ListResult, error)
	GetConnectionDetails(ctx context.Context, r *http.Request) (*allocation.AllocationChangeCollector, error)
	GetLatestReadMarker(ctx context.Context, r *http.Request) (*readmarker.ReadMarker, error)
	GetObjectPathFromBlockNum(ctx context.Context, r *http.Request) (*ObjectPathResult, error)
	GetReferencePath(ctx context.Context, r *http.Request) (*ReferencePathResult, error)
	AcceptChallenge(ctx context.Context, r *http.Request) (interface{}, error)
	// ChallengeData(r *http.Request) (string, error)
}

//SHandler - Singleton for the storage handler
var SHandler StorageHandler
var metaDataStore datastore.Store
var fileStore filestore.FileStore

/*GetStorageHandler - get the storage handler that is setup */
func GetStorageHandler() StorageHandler {
	return SHandler
}

func GetMetaDataStore() datastore.Store {
	return metaDataStore
}
