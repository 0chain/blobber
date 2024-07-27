package reference

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

type QueryCollector interface {
	CreateRefRecord(ref *Ref)
	DeleteRefRecord(ref *Ref)
	Finalize(ctx context.Context) error
	AddToCache(ref *Ref)
	GetFromCache(lookupHash string) *Ref
}

type dbCollector struct {
	createdRefs []*Ref
	deletedRefs []*Ref
	refMap      map[string]*Ref
}

func NewCollector(changes int) QueryCollector {
	return &dbCollector{
		createdRefs: make([]*Ref, 0, changes*4),
		deletedRefs: make([]*Ref, 0, changes*4),
		refMap:      make(map[string]*Ref),
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
		err := db.Delete(dc.deletedRefs).Error
		if err != nil {
			return err
		}
	}
	if len(dc.createdRefs) > 0 {
		err := db.Create(dc.createdRefs).Error
		if err != nil {
			return err
		}
	}

	return nil
}

func (dc *dbCollector) AddToCache(ref *Ref) {
	dc.refMap[ref.LookupHash] = ref
}

func (dc *dbCollector) GetFromCache(lookupHash string) *Ref {
	return dc.refMap[lookupHash]
}
