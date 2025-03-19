package reference

import (
	"context"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type QueryCollector interface {
	CreateRefRecord(ref *Ref)
	DeleteRefRecord(ref *Ref)
	Finalize(ctx context.Context, allocationID string, allocationVersion int64) error
	AddToCache(ref *Ref)
	GetFromCache(lookupHash string) *Ref
	DeleteLookupRefRecord(ref *Ref)
	LockTransaction()
	UnlockTransaction()
}

type dbCollector struct {
	createdRefs []*Ref
	deletedRefs []*Ref
	refCache    RefCache
	refMap      map[string]*Ref
	txnLock     sync.Mutex
	lock        sync.Mutex
}

type RefCache struct {
	AllocationVersion int64
	CreatedRefs       []*Ref
	DeletedRefs       []*Ref
}

var (
	cacheMap     = make(map[string]*RefCache)
	cacheMapLock sync.RWMutex
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
	dc.lock.Lock()
	dc.createdRefs = append(dc.createdRefs, ref)
	if ref.Type == FILE {
		dc.refCache.CreatedRefs = append(dc.refCache.CreatedRefs, ref)
	}
	dc.lock.Unlock()
}

func (dc *dbCollector) DeleteRefRecord(ref *Ref) {
	dc.lock.Lock()
	dc.deletedRefs = append(dc.deletedRefs, ref)
	if ref.Type == FILE {
		dc.refCache.DeletedRefs = append(dc.refCache.DeletedRefs, ref)
	}
	dc.lock.Unlock()
}

func (dc *dbCollector) DeleteLookupRefRecord(ref *Ref) {
	dc.refCache.DeletedRefs = append(dc.refCache.DeletedRefs, ref)
}

func (dc *dbCollector) Finalize(ctx context.Context, allocationID string, allocationVersion int64) error {
	db := datastore.GetStore().GetTransaction(ctx)
	if len(dc.deletedRefs) > 0 {
		err := db.Delete(&(dc.deletedRefs)).Error
		if err != nil {
			return err
		}
	}
	if len(dc.createdRefs) > 0 {
		err := db.Create(&(dc.createdRefs)).Error
		if err != nil {
			for ind, ref := range dc.createdRefs {
				logging.Logger.Error("create_ref_error", zap.String("lookup_hash", ref.LookupHash), zap.String("path", ref.Path), zap.Int("index", ind), zap.Int64("allocation_version", allocationVersion))
			}
			return err
		}
	}
	dc.refCache.AllocationVersion = allocationVersion
	cacheMapLock.Lock()
	cacheMap[allocationID] = &(dc.refCache)
	logging.Logger.Info("Finalize", zap.Int("created", len(dc.createdRefs)), zap.Int("deleted", len(dc.deletedRefs)), zap.Int64("allocation_version", cacheMap[allocationID].AllocationVersion), zap.String("allocation_id", allocationID))
	cacheMapLock.Unlock()
	return nil
}

func (dc *dbCollector) AddToCache(ref *Ref) {
	dc.lock.Lock()
	dc.refMap[ref.LookupHash] = ref
	dc.lock.Unlock()
}

func (dc *dbCollector) GetFromCache(lookupHash string) *Ref {
	dc.lock.Lock()
	defer dc.lock.Unlock()
	return dc.refMap[lookupHash]
}

func GetRefCache(allocationID string) *RefCache {
	cacheMapLock.RLock()
	defer cacheMapLock.RUnlock()
	return cacheMap[allocationID]
}

func DeleteRefCache(allocationID string) {
	cacheMapLock.Lock()
	cacheMap[allocationID] = nil
	cacheMapLock.Unlock()
}

func (dc *dbCollector) LockTransaction() {
	dc.txnLock.Lock()
}

func (dc *dbCollector) UnlockTransaction() {
	dc.txnLock.Unlock()
}
