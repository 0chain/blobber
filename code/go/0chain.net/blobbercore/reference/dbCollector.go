package reference

import (
	"context"
	"sync"

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
	mut         sync.Mutex
}

func NewCollector(changes int) QueryCollector {
	return &dbCollector{
		createdRefs: make([]*Ref, 0, changes*4),
		deletedRefs: make([]*Ref, 0, changes*4),
	}
}

func (dc *dbCollector) CreateRefRecord(ref *Ref) {
	dc.mut.Lock()
	dc.createdRefs = append(dc.createdRefs, ref)
	dc.mut.Unlock()
}

func (dc *dbCollector) DeleteRefRecord(ref *Ref) {
	dc.mut.Lock()
	dc.deletedRefs = append(dc.deletedRefs, ref)
	dc.mut.Unlock()
}

func (dc *dbCollector) Finalize(ctx context.Context) error {
	db := datastore.GetStore().GetTransaction(ctx)
	if len(dc.deletedRefs) > 0 {
		err := db.Delete(dc.deletedRefs).Error
		if err != nil {
			return err
		}
	}
	err := db.Create(dc.createdRefs).Error
	if err != nil {
		return err
	}

	return nil
}
