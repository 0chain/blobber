package reference

import (
	"context"

	"0chain.net/common"
	"0chain.net/datastore"
)

type ContentReference struct {
	AllocationID   string           `json:"allocation_id"`
	ContentHash    string           `json:"content_hash"`
	ReferenceCount int64            `json:"ref_count"`
	LastUpdated    common.Timestamp `json:"last_updated"`
}

var contentRefMetaData *datastore.EntityMetadataImpl

/*Provider - entity provider for client object */
func ContentReferenceProvider() datastore.Entity {
	t := &ContentReference{}
	return t
}

func SetupContentReferenceEntity(store datastore.Store) {
	contentRefMetaData = datastore.MetadataProvider()
	contentRefMetaData.Name = "contentref"
	contentRefMetaData.DB = "contentref"
	contentRefMetaData.Provider = ContentReferenceProvider
	contentRefMetaData.Store = store

	datastore.RegisterEntityMetadata("contentref", contentRefMetaData)
}

func (fr *ContentReference) GetEntityMetadata() datastore.EntityMetadata {
	return contentRefMetaData
}
func (fr *ContentReference) SetKey(key datastore.Key) {
	//wm.ID = datastore.ToString(key)
}

func (fr *ContentReference) GetKey() string {
	return fr.GetEntityMetadata().GetDBName() + ":" + GetReferenceLookup(fr.AllocationID, fr.ContentHash)
}

func (fr *ContentReference) Read(ctx context.Context, key datastore.Key) error {
	return contentRefMetaData.GetStore().Read(ctx, key, fr)
}
func (fr *ContentReference) Write(ctx context.Context) error {
	fr.LastUpdated = common.Now()
	return contentRefMetaData.GetStore().Write(ctx, fr)
}
func (fr *ContentReference) Delete(ctx context.Context) error {
	return contentRefMetaData.GetStore().Delete(ctx, fr)
}

func UpdateContentRefForWrite(ctx context.Context, allocationID string, contentHash string) error {
	contentRef := ContentReferenceProvider().(*ContentReference)
	contentRef.AllocationID = allocationID
	contentRef.ContentHash = contentHash
	err := contentRef.Read(ctx, contentRef.GetKey())
	if err != nil && err != datastore.ErrKeyNotFound {
		return err
	}
	contentRef.ReferenceCount++
	return contentRef.Write(ctx)
}

func UpdateContentRefForDelete(ctx context.Context, allocationID string, contentHash string) error {
	contentRef := ContentReferenceProvider().(*ContentReference)
	contentRef.AllocationID = allocationID
	contentRef.ContentHash = contentHash
	err := contentRef.Read(ctx, contentRef.GetKey())
	if err != nil {
		return err
	}
	contentRef.ReferenceCount--
	return contentRef.Write(ctx)
}
