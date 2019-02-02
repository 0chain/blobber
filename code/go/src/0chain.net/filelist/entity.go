package filelist

import (
	"context"

	"0chain.net/cache"
	"0chain.net/datastore"
	"github.com/blobber/code/go/src/0chain.net/encryption"
)

type BlobberFileList struct {
	AllocationID string         `json:"allocation_id"`
	List         []*BlobberFile `json:"list"`
}

type BlobberFile struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	ContentHash string `json:"content_hash"`
}

var fileListEntityMetaData *datastore.EntityMetadataImpl
var allocationFileListCache = cache.NewLRUCache(1000)

func NewBlobberFileList(allocationID string) *BlobberFileList {
	entity := BlobberFileListProvider().(*BlobberFileList)
	entity.AllocationID = allocationID
	return entity
}

/*Provider - entity provider for client object */
func BlobberFileListProvider() datastore.Entity {
	t := &BlobberFileList{}
	return t
}

func SetupEntity(store datastore.Store) {
	fileListEntityMetaData = datastore.MetadataProvider()
	fileListEntityMetaData.Name = "bl"
	fileListEntityMetaData.DB = "bl"
	fileListEntityMetaData.Provider = BlobberFileListProvider
	fileListEntityMetaData.Store = store

	datastore.RegisterEntityMetadata(fileListEntityMetaData.GetDBName(), fileListEntityMetaData)
}

func (bl *BlobberFileList) GetEntityMetadata() datastore.EntityMetadata {
	return fileListEntityMetaData
}
func (bl *BlobberFileList) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}

func (bl *BlobberFileList) GetKey() datastore.Key {
	return datastore.ToKey(fileListEntityMetaData.GetDBName() + ":" + bl.AllocationID)
}

func (bl *BlobberFileList) Read(ctx context.Context, key datastore.Key) error {
	return fileListEntityMetaData.GetStore().Read(ctx, key, bl)
}
func (bl *BlobberFileList) Write(ctx context.Context) error {
	allocationFileListCache.Delete(bl.AllocationID)
	return fileListEntityMetaData.GetStore().Write(ctx, bl)
}
func (bl *BlobberFileList) Delete(ctx context.Context) error {
	return nil
}

func (bf *BlobberFile) GetHashData() string {
	return bf.Name + ":" + bf.Path + ":" + bf.ContentHash
}

func (bf *BlobberFile) GetHash() string {
	return encryption.Hash(bf.GetHashData())
}

func (bf *BlobberFile) GetHashBytes() []byte {
	return encryption.RawHash(bf.GetHashData())
}

func GetFileList(ctx context.Context, allocationID string) (*BlobberFileList, error) {
	cachedList, err := allocationFileListCache.Get(allocationID)
	fileList := cachedList.(*BlobberFileList)
	if err != nil {
		fileList = NewBlobberFileList(allocationID)
		err = fileList.Read(ctx, fileList.GetKey())
		if err != nil {
			return nil, err
		}
		allocationFileListCache.Add(allocationID, fileList)
	}
	return fileList, nil
}
