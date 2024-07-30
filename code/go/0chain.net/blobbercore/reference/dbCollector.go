package reference

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
)

type QueryCollector interface {
	CreateRefRecord(ref *Ref)
	DeleteRefRecord(ref *Ref)
	Finalize(ctx context.Context, allocationID string, allocationVersion int64) error
	AddToCache(ref *Ref)
	GetFromCache(lookupHash string) *Ref
	DeleteLookupRefRecord(ref *Ref)
}

type dbCollector struct {
	createdRefs []*Ref
	deletedRefs []*Ref
	refCache    RefCache
	refMap      map[string]*Ref
}

type RefCache struct {
	AllocationVersion int64
	CreatedRefs       []*Ref
	DeletedRefs       []*Ref
}

var (
	cacheMap = make(map[string]*RefCache)
)

func NewCollector(changes int) QueryCollector {
	return &dbCollector{
		createdRefs: make([]*Ref, 0, changes*2),
		deletedRefs: make([]*Ref, 0, changes*2),
		refCache: RefCache{
			CreatedRefs: make([]*Ref, 0, changes),
			DeletedRefs: make([]*Ref, 0, changes),
		},
		refMap: make(map[string]*Ref),
	}
}

func (dc *dbCollector) CreateRefRecord(ref *Ref) {
	dc.createdRefs = append(dc.createdRefs, ref)
	if ref.Type == FILE {
		dc.refCache.CreatedRefs = append(dc.refCache.CreatedRefs, ref)
	}
}

func (dc *dbCollector) DeleteRefRecord(ref *Ref) {
	dc.deletedRefs = append(dc.deletedRefs, ref)
	if ref.Type == FILE {
		dc.refCache.DeletedRefs = append(dc.refCache.DeletedRefs, ref)
	}
}

func (dc *dbCollector) DeleteLookupRefRecord(ref *Ref) {
	dc.refCache.DeletedRefs = append(dc.refCache.DeletedRefs, ref)
}

func (dc *dbCollector) Finalize(ctx context.Context, allocationID string, allocationVersion int64) error {
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
	dc.refCache.AllocationVersion = allocationVersion
	cacheMap[allocationID] = &dc.refCache
	return nil
}

func (dc *dbCollector) AddToCache(ref *Ref) {
	dc.refMap[ref.LookupHash] = ref
}

func (dc *dbCollector) GetFromCache(lookupHash string) *Ref {
	return dc.refMap[lookupHash]
}

func GetRefCache(allocationID string) *RefCache {
	return cacheMap[allocationID]
}

func DeleteRefCache(allocationID string) {
	cacheMap[allocationID] = nil
}
