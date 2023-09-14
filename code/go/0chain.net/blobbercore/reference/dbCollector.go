package reference

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

type QueryCollector interface {
	CreateRefRecord(ref *Ref)
	DeleteRefRecord(ref *Ref)
	Finalize(ctx context.Context) error
}

type dbCollector struct {
	createdRefs []*Ref
	deletedRefs []*Ref
}

func NewCollector(changes int) QueryCollector {
	return &dbCollector{
		createdRefs: make([]*Ref, 0, changes*4),
		deletedRefs: make([]*Ref, 0, changes*4),
	}
}

func (dc *dbCollector) CreateRefRecord(ref *Ref) {
	dc.createdRefs = append(dc.createdRefs, ref)
}

func (dc *dbCollector) DeleteRefRecord(ref *Ref) {
	dc.deletedRefs = append(dc.deletedRefs, ref)
}

func (dc *dbCollector) Finalize(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	if len(dc.deletedRefs) > 0 {
		err := db.Delete(&dc.deletedRefs).Error
		if err != nil {
			return err
		}
	}
	err := db.Create(&dc.createdRefs).Error
	if err != nil {
		return err
	}
	for _, ref := range dc.createdRefs {
		switch ref.Type {
		case FILE:
			if ref.prevID != 0 {
				FileUpdated(ctx, ref.prevID, ref.ID)
			} else {
				NewFileCreated(ctx, ref.ID)
			}
		default:
			if ref.prevID == 0 {
				NewDirCreated(ctx, ref.ID)
			}
		}
	}
	return nil
}
