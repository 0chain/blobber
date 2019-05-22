package reference

import (
	"context"
)

/*
type ContentReference struct {
	AllocationID   string `gorm:"column:allocation_id"`
	ContentHash    string `gorm:"column:content_hash"`
	ReferenceCount int64  `gorm:"column:ref_count"`
	datastore.ModelWithTS
}*/

func UpdateContentRefForWrite(ctx context.Context, allocationID string, contentHash string) error {
	// contentRef := ContentReferenceProvider().(*ContentReference)
	// contentRef.AllocationID = allocationID
	// contentRef.ContentHash = contentHash
	// err := contentRef.Read(ctx, contentRef.GetKey())
	// if err != nil && err != datastore.ErrKeyNotFound {
	// 	return err
	// }
	// contentRef.ReferenceCount++
	// return contentRef.Write(ctx)
	return nil
}

func UpdateContentRefForDelete(ctx context.Context, allocationID string, contentHash string) error {
	// contentRef := ContentReferenceProvider().(*ContentReference)
	// contentRef.AllocationID = allocationID
	// contentRef.ContentHash = contentHash
	// err := contentRef.Read(ctx, contentRef.GetKey())
	// if err != nil {
	// 	return err
	// }
	// contentRef.ReferenceCount--
	// return contentRef.Write(ctx)
	return nil
}
