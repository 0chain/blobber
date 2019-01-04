package blobber

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/filestore"
	"0chain.net/readmarker"
	"0chain.net/reference"
	"0chain.net/writemarker"
)

const (
	INSERT_OPERATION = "insert"
	DELETE_OPERATION = "delete"
)

type AllocationChangeCollector struct {
	ConnectionID string                       `json:"connection_id"`
	AllocationID string                       `json:"allocation_id"`
	ClientID     string                       `json:"client_id"`
	Size         int64                        `json:"size"`
	LastUpdated  common.Timestamp             `json:"last_updated"`
	Changes      []*AllocationChange          `json:"changes"`
	ChangeMap    map[string]*AllocationChange `json:"-"`
}

type AllocationChange struct {
	*UploadFormData
	Size      int64  `json:"size"`
	Operation string `json:"operation"`
}

var allocationChangeEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func AllocationChangeCollectorProvider() datastore.Entity {
	t := &AllocationChangeCollector{}
	t.ChangeMap = make(map[string]*AllocationChange, 0)
	return t
}

func SetupAllocationChangeCollectorEntity(store datastore.Store) {
	allocationChangeEntityMetaData = datastore.MetadataProvider()
	allocationChangeEntityMetaData.Name = "allocation_change"
	allocationChangeEntityMetaData.DB = "allocation_change"
	allocationChangeEntityMetaData.Provider = AllocationChangeCollectorProvider
	allocationChangeEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("allocation_change", allocationChangeEntityMetaData)
}

func (a *AllocationChangeCollector) GetEntityMetadata() datastore.EntityMetadata {
	return allocationChangeEntityMetaData
}
func (a *AllocationChangeCollector) SetKey(key datastore.Key) {
	//a.ID = datastore.ToString(key)
}
func (a *AllocationChangeCollector) GetKey() datastore.Key {
	return datastore.ToKey(allocationChangeEntityMetaData.GetDBName() + ":" + encryption.Hash(a.AllocationID+":"+a.ConnectionID))
}
func (a *AllocationChangeCollector) Read(ctx context.Context, key datastore.Key) error {
	defer a.ComputeChangeMap()
	return allocationChangeEntityMetaData.GetStore().Read(ctx, key, a)
}
func (a *AllocationChangeCollector) Write(ctx context.Context) error {
	return allocationChangeEntityMetaData.GetStore().Write(ctx, a)
}
func (a *AllocationChangeCollector) Delete(ctx context.Context) error {
	return allocationChangeEntityMetaData.GetStore().Delete(ctx, a)
}
func (a *AllocationChangeCollector) ComputeChangeMap() {
	for _, element := range a.Changes {
		key := reference.GetReferenceLookup(a.AllocationID, element.Path)
		a.ChangeMap[key] = element
	}
}

func (a *AllocationChangeCollector) AddChange(change *AllocationChange) {
	a.Changes = append(a.Changes, change)
	key := reference.GetReferenceLookup(a.AllocationID, change.Path)
	a.ChangeMap[key] = change
}

func (a *AllocationChangeCollector) DeleteChanges(ctx context.Context) error {
	for _, change := range a.Changes {
		if change.Operation == INSERT_OPERATION {
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = change.Filename
			fileInputData.Path = change.Path
			fileInputData.Hash = change.Hash
			err := fileStore.DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
			if err != nil {
				return err
			}
		}
	}
	return a.Delete(ctx)
}

func (a *AllocationChangeCollector) ApplyChanges(ctx context.Context) (*reference.Ref, error) {
	for _, change := range a.Changes {
		if change.Operation == INSERT_OPERATION {
			fileref := reference.FileRefProvider().(*reference.FileRef)
			fileref.AllocationID = a.AllocationID
			fileref.Name = change.Filename
			fileref.Path = change.Path
			fileref.Size = change.Size
			fileref.Type = reference.FILE
			fileref.ContentHash = change.Hash
			fileref.CustomMeta = change.CustomMeta
			fileref.ActualFileSize = change.ActualSize
			fileref.ActualFileHash = change.ActualHash
			fileref.MerkleRoot = change.MerkleRoot
			fileref.CalculateHash(ctx)
			parentdir, _ := filepath.Split(change.Path)
			parentdir = filepath.Clean(parentdir)

			parentRef := reference.RefProvider().(*reference.Ref)
			parentRef.AllocationID = a.AllocationID
			parentRef.Path = parentdir
			fileref.ParentRef = parentRef.GetKey()

			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = fileref.Name
			fileInputData.Path = fileref.Path
			fileInputData.Hash = fileref.ContentHash

			err := fileref.Write(ctx)
			if err != nil {
				return nil, common.NewError("fileref_write_error", "Error writing the file meta info. "+err.Error())
			}
			err = reference.CreateDirRefsIfNotExists(ctx, a.AllocationID, parentdir, fileref.GetKey())
			if err != nil {
				return nil, common.NewError("create_ref_error", "Error creating the dir meta info. "+err.Error())
			}
			err = parentRef.Read(ctx, parentRef.GetKey())
			if err != nil {
				return nil, common.NewError("parent_ref_not_found", "Parent dir meta data not found. "+err.Error())
			}
			fmt.Println(parentRef.GetKey() + ", " + parentRef.Path + ", " + strings.Join(parentRef.ChildRefs, ","))
			err = reference.RecalculateHashBottomUp(ctx, parentRef)
			if err != nil {
				return nil, common.NewError("allocation_hash_error", "Error calculating the allocation hash. "+err.Error())
			}

			err = fileStore.CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
			if err != nil {
				return nil, common.NewError("file_store_error", "Error committing to file store. "+err.Error())
			}
		}
	}
	rootRef, err := reference.GetRootReference(ctx, a.AllocationID)
	if err != nil {
		return nil, common.NewError("root_ref_read_error", "Error getting the root reference. "+err.Error())
	}
	return rootRef, nil
}

type UploadFormData struct {
	ConnectionID string `json:"connection_id"`
	Filename     string `json:"filename"`
	Path         string `json:"filepath"`
	Hash         string `json:"content_hash"`
	MerkleRoot   string `json:"merkle_root"`
	ActualHash   string `json:"actual_hash"`
	ActualSize   int64  `json:"actual_size"`
	CustomMeta   string `json:"custom_meta"`
}

type UploadResult struct {
	Filename   string `json:"filename"`
	Size       int64  `json:"size"`
	Hash       string `json:"content_hash"`
	MerkleRoot string `json:"merkle_root"`
}

type CommitResult struct {
	AllocationRoot string                   `json:"allocation_root"`
	WriteMarker    *writemarker.WriteMarker `json:"write_marker"`
	Success        bool                     `json:"success"`
	ErrorMessage   string                   `json:"error_msg,omitempty"`
	//Result         []*UploadResult         `json:"result"`
}

type ListResult struct {
	AllocationRoot string                   `json:"allocation_root"`
	Meta           map[string]interface{}   `json:"meta_data"`
	Entities       []map[string]interface{} `json:"list"`
}

type DownloadResponse struct {
	Data []byte `json:"data"`
}

//StorageHandler - interfact for handling storage requests
type StorageHandler interface {
	WriteFile(ctx context.Context, r *http.Request) (*UploadResult, error)
	CommitWrite(ctx context.Context, r *http.Request) (*CommitResult, error)
	DownloadFile(ctx context.Context, r *http.Request) (*DownloadResponse, error)
	GetFileMeta(ctx context.Context, r *http.Request) (interface{}, error)
	ListEntities(ctx context.Context, r *http.Request) (*ListResult, error)
	GetConnectionDetails(ctx context.Context, r *http.Request) (*AllocationChangeCollector, error)
	GetLatestReadMarker(ctx context.Context, r *http.Request) (*readmarker.ReadMarker, error)
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
