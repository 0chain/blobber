package allocation

import (
	"context"

	"0chain.net/datastore"
)

type AllocationStatus struct {
	ID                    string `json:"id"`
	LastCommittedWMEntity string `json:"last_committed_write_marker"`
}

var allocationStatusMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func AllocationStatusProvider() datastore.Entity {
	t := &AllocationStatus{}
	return t
}

func SetupAllocationStatusEntity(store datastore.Store) {
	allocationStatusMetaData = datastore.MetadataProvider()
	allocationStatusMetaData.Name = "allocation_status"
	allocationStatusMetaData.DB = "allocation_status"
	allocationStatusMetaData.Provider = Provider
	allocationStatusMetaData.Store = store

	datastore.RegisterEntityMetadata("allocation_status", allocationStatusMetaData)
}

func (a *AllocationStatus) GetEntityMetadata() datastore.EntityMetadata {
	return allocationStatusMetaData
}
func (a *AllocationStatus) SetKey(key datastore.Key) {
	a.ID = datastore.ToString(key)
}
func (a *AllocationStatus) GetKey() datastore.Key {
	return datastore.ToKey(a.GetEntityMetadata().GetDBName() + ":" + a.ID)
}
func (a *AllocationStatus) Read(ctx context.Context, key datastore.Key) error {
	return allocationStatusMetaData.GetStore().Read(ctx, key, a)
}
func (a *AllocationStatus) Write(ctx context.Context) error {
	return allocationStatusMetaData.GetStore().Write(ctx, a)
}
func (a *AllocationStatus) Delete(ctx context.Context) error {
	return nil
}
