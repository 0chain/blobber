package allocation

import (
	"context"
	"path/filepath"

	"0chain.net/common"
	"0chain.net/datastore"
	"0chain.net/encryption"
	"0chain.net/filestore"
	"0chain.net/reference"
)

const (
	INSERT_OPERATION = "insert"
	DELETE_OPERATION = "delete"
	UPDATE_OPERATION = "update"
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

type UploadFormData struct {
	ConnectionID string `json:"connection_id"`
	Filename     string `json:"filename"`
	Path         string `json:"filepath"`
	Hash         string `json:"content_hash,omitempty"`
	MerkleRoot   string `json:"merkle_root,omitempty"`
	ActualHash   string `json:"actual_hash,omitempty"`
	ActualSize   int64  `json:"actual_size,omitempty"`
	CustomMeta   string `json:"custom_meta,omitempty"`
}

type AllocationChange struct {
	*UploadFormData
	DeleteToken string `json:"delete_token"`
	Size        int64  `json:"size"`
	NewFileSize int64  `json:"new_size"`
	NumBlocks   int64  `json:"num_of_blocks"`
	Operation   string `json:"operation"`
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

func (a *AllocationChangeCollector) DeleteChanges(ctx context.Context, fileStore filestore.FileStore) error {
	for _, change := range a.Changes {
		if change.Operation == INSERT_OPERATION || change.Operation == UPDATE_OPERATION {
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = change.Filename
			fileInputData.Path = change.Path
			fileInputData.Hash = change.Hash
			fileStore.DeleteTempFile(a.AllocationID, fileInputData, a.ConnectionID)
		}
		if change.Operation == DELETE_OPERATION {
			deleteToken := DeleteTokenProvider().(*DeleteToken)
			err := deleteToken.Read(ctx, change.DeleteToken)
			if err == nil && deleteToken.Status == NEW {
				err = deleteToken.Delete(ctx)
				if err != nil {
					return common.NewError("delete_token_remove_error", "Error removing the delete token."+err.Error())
				}
			}
		}
	}
	return a.Delete(ctx)
}

func (a *AllocationChangeCollector) CommitToFileStore(ctx context.Context, fileStore filestore.FileStore) error {
	for _, change := range a.Changes {
		if fileStore != nil && (change.Operation == INSERT_OPERATION || change.Operation == UPDATE_OPERATION) {
			fileInputData := &filestore.FileInputData{}
			fileInputData.Name = change.Filename
			fileInputData.Path = change.Path
			fileInputData.Hash = change.Hash
			_, err := fileStore.CommitWrite(a.AllocationID, fileInputData, a.ConnectionID)
			if err != nil {
				return common.NewError("file_store_error", "Error committing to file store. "+err.Error())
			}
		}
	}
	return nil
}

func (a *AllocationChangeCollector) ApplyChanges(ctx context.Context, fileStore filestore.FileStore, dbStore datastore.Store, writeMarkerKey string) (*reference.Ref, error) {
	if dbStore == nil {
		dbStore = a.GetEntityMetadata().GetStore()
	}

	for _, change := range a.Changes {
		if change.Operation == DELETE_OPERATION {
			fileref := reference.FileRefProvider().(*reference.FileRef)
			fileref.AllocationID = a.AllocationID
			fileref.Name = change.Filename
			fileref.Path = change.Path
			err := dbStore.Read(ctx, fileref.GetKey(), fileref)
			if err != nil {
				return nil, common.NewError("file_ref_lookup_error", "Error looking up for the file reference."+err.Error())
			}

			err = dbStore.Delete(ctx, fileref)
			if err != nil {
				return nil, common.NewError("file_ref_delete_error", "Error deleting the file reference."+err.Error())
			}
			parentRef := reference.RefProvider().(*reference.Ref)
			err = dbStore.Read(ctx, fileref.ParentRef, parentRef)
			if err != nil {
				return nil, common.NewError("parent_ref_lookup_error", "Error looking up for the parent of the file reference."+err.Error())
			}

			parentRef.DeleteChild(fileref.GetKey())

			err = reference.RecalculateHashBottomUp(ctx, parentRef, dbStore)
			if err != nil {
				return nil, common.NewError("allocation_hash_error", "Error calculating the allocation hash. "+err.Error())
			}
			if dbStore == a.GetEntityMetadata().GetStore() {
				deleteToken := DeleteTokenProvider().(*DeleteToken)
				deleteToken.FileRefHash = fileref.Hash
				err = dbStore.Read(ctx, deleteToken.GetKey(), deleteToken)
				if err != nil {
					return nil, common.NewError("delete_token_read_error", "Error reading the delete token."+err.Error())
				}
				deleteToken.Status = COMMITTED
				err = dbStore.Write(ctx, deleteToken)
				if err != nil {
					return nil, common.NewError("delete_token_update_error", "Error updating the delete token status."+err.Error())
				}

				err = reference.UpdateContentRefForDelete(ctx, a.AllocationID, fileref.ContentHash)
				if err != nil {
					return nil, common.NewError("content_ref_write_error", "Errorn updating the content ref count")
				}
			}

		} else if change.Operation == INSERT_OPERATION {
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
			fileref.WriteMarker = writeMarkerKey
			fileref.CalculateHash(ctx, dbStore)
			parentdir, _ := filepath.Split(change.Path)
			parentdir = filepath.Clean(parentdir)

			parentRef := reference.RefProvider().(*reference.Ref)
			parentRef.AllocationID = a.AllocationID
			parentRef.Path = parentdir
			fileref.ParentRef = parentRef.GetKey()

			err := dbStore.Write(ctx, fileref)
			if err != nil {
				return nil, common.NewError("fileref_write_error", "Error writing the file meta info. "+err.Error())
			}
			err = reference.CreateDirRefsIfNotExists(ctx, a.AllocationID, parentdir, fileref.GetKey(), dbStore)
			if err != nil {
				return nil, common.NewError("create_ref_error", "Error creating the dir meta info. "+err.Error())
			}
			err = dbStore.Read(ctx, parentRef.GetKey(), parentRef)
			if err != nil {
				return nil, common.NewError("parent_ref_not_found", "Parent dir meta data not found. "+err.Error())
			}
			//fmt.Println(parentRef.GetKey() + ", " + parentRef.Path + ", " + strings.Join(parentRef.ChildRefs, ","))
			err = reference.RecalculateHashBottomUp(ctx, parentRef, dbStore)
			if err != nil {
				return nil, common.NewError("allocation_hash_error", "Error calculating the allocation hash. "+err.Error())
			}

			if dbStore == a.GetEntityMetadata().GetStore() {
				err = reference.UpdateContentRefForWrite(ctx, a.AllocationID, change.Hash)
				if err != nil {
					return nil, common.NewError("content_ref_write_error", "Error updating the content ref count")
				}
			}
		} else if change.Operation == UPDATE_OPERATION {
			fileref := reference.FileRefProvider().(*reference.FileRef)
			fileref.AllocationID = a.AllocationID
			fileref.Name = change.Filename
			fileref.Path = change.Path
			err := dbStore.Read(ctx, fileref.GetKey(), fileref)
			if err != nil {
				return nil, common.NewError("file_ref_lookup_error", "Error looking up for the file reference."+err.Error())
			}
			fileref.Size = change.Size + fileref.Size
			origContentHash := fileref.ContentHash
			fileref.ContentHash = change.Hash
			fileref.CustomMeta = change.CustomMeta
			fileref.ActualFileSize = change.ActualSize
			fileref.ActualFileHash = change.ActualHash
			fileref.MerkleRoot = change.MerkleRoot
			fileref.WriteMarker = writeMarkerKey
			fileref.CalculateHash(ctx, dbStore)

			err = dbStore.Write(ctx, fileref)
			if err != nil {
				return nil, common.NewError("fileref_write_error", "Error writing the file meta info. "+err.Error())
			}

			parentRef := reference.RefProvider().(*reference.Ref)
			err = dbStore.Read(ctx, fileref.ParentRef, parentRef)
			if err != nil {
				return nil, common.NewError("parent_ref_not_found", "Parent dir meta data not found. "+err.Error())
			}
			//fmt.Println(parentRef.GetKey() + ", " + parentRef.Path + ", " + strings.Join(parentRef.ChildRefs, ","))
			err = reference.RecalculateHashBottomUp(ctx, parentRef, dbStore)
			if err != nil {
				return nil, common.NewError("allocation_hash_error", "Error calculating the allocation hash. "+err.Error())
			}

			if dbStore == a.GetEntityMetadata().GetStore() {
				err = reference.UpdateContentRefForDelete(ctx, a.AllocationID, origContentHash)
				if err != nil {
					return nil, common.NewError("content_ref_write_error", "Error updating the content ref count for delete."+err.Error())
				}
				err = reference.UpdateContentRefForWrite(ctx, a.AllocationID, change.Hash)
				if err != nil {
					return nil, common.NewError("content_ref_write_error", "Error updating the content ref count for update."+err.Error())
				}
			}
		}
	}
	rootRef, err := reference.GetRootReferenceFromStore(ctx, a.AllocationID, dbStore)
	if err != nil {
		return nil, common.NewError("root_ref_read_error", "Error getting the root reference. "+err.Error())
	}
	return rootRef, nil
}
