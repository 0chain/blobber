package allocation

import (
	"context"

	"0chain.net/common"
	"0chain.net/datastore"
)

type Allocation struct {
	Version         string           `json:"version"`
	ID              string           `json:"id"`
	TotalSize       int64            `json:"size"`
	UsedSize        int64            `json:"used_size"`
	OwnerID         string           `json:"owner_id"`
	OwnerPublicKey  string           `json:"owner_public_key"`
	Expiration      common.Timestamp `json:"expiration_date"`
	AllocationRoot  string           `json:"allocation_root"`
	BlobberSize     int64            `json:"blobber_size"`
	BlobberSizeUsed int64            `json:"blobber_size_used"`
	LatestWMEntity  string           `json:"write_marker"`
}

var allocationEntityMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func Provider() datastore.Entity {
	t := &Allocation{}
	t.Version = "1.0"
	return t
}

func SetupAllocationEntity(store datastore.Store) {
	allocationEntityMetaData = datastore.MetadataProvider()
	allocationEntityMetaData.Name = "allocation"
	allocationEntityMetaData.Provider = Provider
	allocationEntityMetaData.Store = store

	datastore.RegisterEntityMetadata("allocation", allocationEntityMetaData)
	SetupAllocationStatusEntity(store)
}

func (a *Allocation) GetEntityMetadata() datastore.EntityMetadata {
	return allocationEntityMetaData
}
func (a *Allocation) SetKey(key datastore.Key) {
	a.ID = datastore.ToString(key)
}
func (a *Allocation) GetKey() datastore.Key {
	return datastore.ToKey("allocation:" + a.ID)
}
func (a *Allocation) Read(ctx context.Context, key datastore.Key) error {
	return allocationEntityMetaData.GetStore().Read(ctx, key, a)
}
func (a *Allocation) Write(ctx context.Context) error {
	return allocationEntityMetaData.GetStore().Write(ctx, a)
}
func (a *Allocation) Delete(ctx context.Context) error {
	return nil
}
