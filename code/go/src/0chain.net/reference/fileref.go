package reference

import (
	"context"
	"math"
	"strconv"
	"strings"

	"0chain.net/encryption"

	"0chain.net/common"
	"0chain.net/datastore"
)

type FileRef struct {
	Ref
	CustomMeta     string `json:"custom_meta" list:"custom_meta"`
	ContentHash    string `json:"content_hash" list:"content_hash"`
	Size           int64  `json:"size" list:"size"`
	MerkleRoot     string `json:"merkle_root" list:"merkle_root"`
	ActualFileSize int64  `json:"actual_file_size" list:"actual_file_size"`
	ActualFileHash string `json:"actual_file_hash" list:"actual_file_hash"`
	WriteMarker    string `json:"write_marker"`
}

const CHUNK_SIZE = 64 * 1024

var fileRefEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func FileRefProvider() datastore.Entity {
	t := &FileRef{}
	t.Version = "1.0"
	t.CreationDate = common.Now()
	t.Type = FILE
	return t
}

func SetupFileRefEntity(store datastore.Store) {
	fileRefEntityMetaData = datastore.MetadataProvider()
	fileRefEntityMetaData.Name = "fileref"
	fileRefEntityMetaData.DB = "fileref"
	fileRefEntityMetaData.Provider = FileRefProvider
	fileRefEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("fileref", fileRefEntityMetaData)
}

func (fr *FileRef) GetEntityMetadata() datastore.EntityMetadata {
	return fileRefEntityMetaData
}
func (fr *FileRef) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}

func (fr *FileRef) GetKey() string {
	return fr.GetEntityMetadata().GetDBName() + ":" + GetReferenceLookup(fr.AllocationID, fr.Path)
}

func (fr *FileRef) Read(ctx context.Context, key datastore.Key) error {
	return fileRefEntityMetaData.GetStore().Read(ctx, key, fr)
}
func (fr *FileRef) Write(ctx context.Context) error {
	return fileRefEntityMetaData.GetStore().Write(ctx, fr)
}
func (fr *FileRef) Delete(ctx context.Context) error {
	return nil
}

func (fr *FileRef) GetKeyFromPathHash(path_hash string) datastore.Key {
	return fr.GetEntityMetadata().GetDBName() + ":" + path_hash
}

func (fr *FileRef) GetHashData() string {
	hashArray := make([]string, 0)
	hashArray = append(hashArray, fr.AllocationID)
	hashArray = append(hashArray, fr.Type)
	hashArray = append(hashArray, fr.Name)
	hashArray = append(hashArray, fr.Path)
	hashArray = append(hashArray, strconv.FormatInt(fr.Size, 10))
	hashArray = append(hashArray, fr.ContentHash)
	hashArray = append(hashArray, fr.MerkleRoot)
	hashArray = append(hashArray, strconv.FormatInt(fr.ActualFileSize, 10))
	hashArray = append(hashArray, fr.ActualFileHash)
	return strings.Join(hashArray, ":")
}

func (fr *FileRef) GetHash(ctx context.Context) string {
	return fr.Hash
}

func (fr *FileRef) CalculateHash(ctx context.Context, dbStore datastore.Store) (string, error) {
	//fmt.Println("Fileref hash : " + fr.GetHashData())
	fr.Hash = encryption.Hash(fr.GetHashData())
	fr.NumBlocks = int64(math.Ceil(float64(fr.Size*1.0) / CHUNK_SIZE))
	fr.PathHash = GetReferenceLookup(fr.AllocationID, fr.Path)
	return fr.Hash, nil
}

func (fr *FileRef) GetListingData(context.Context) map[string]interface{} {
	return GetListingFieldsMap(*fr)
}

func (fr *FileRef) GetType() string {
	return fr.Type
}

func (fr *FileRef) GetNumBlocks(context.Context) int64 {
	return fr.NumBlocks
}

func (fr *FileRef) GetPathHash() string {
	return fr.PathHash
}

func (fr *FileRef) GetPath() string {
	return fr.Path
}

func (fr *FileRef) GetName() string {
	return fr.Name
}
