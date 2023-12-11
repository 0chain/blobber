package allocation

import (
	"context"
	"fmt"
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SQLWhereGetById = "allocations.id = ?"
	SQLWhereGetByTx = "allocations.tx = ?"
	lruSize         = 100
)

var (
	Repo    *Repository
	mapLock sync.Mutex
)

type AllocationUpdate func(a *Allocation)

func init() {
	allocCache, _ := lru.New[string, Allocation](lruSize)
	Repo = &Repository{
		allocCache: allocCache,
		allocLock:  make(map[string]*sync.Mutex),
	}
}

type Repository struct {
	allocCache *lru.Cache[string, Allocation]
	allocLock  map[string]*sync.Mutex
}

type AllocationCache struct {
	Allocation        *Allocation
	AllocationUpdates []AllocationUpdate
}

type Res struct {
	ID string
}

func (r *Repository) GetById(ctx context.Context, id string) (*Allocation, error) {
	tx := datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	cache, err := getCache(tx)
	if err != nil {
		return nil, err
	}

	if a, ok := cache[id]; ok {
		return a.Allocation, nil
	}

	a := r.getAllocFromGlobalCache(id)
	if a != nil {
		cache[id] = AllocationCache{
			Allocation: a,
		}
		return a, nil
	}

	alloc := &Allocation{}
	err = tx.Table(TableNameAllocation).Where(SQLWhereGetById, id).Take(alloc).Error
	if err != nil {
		return alloc, err
	}

	cache[id] = AllocationCache{
		Allocation: alloc,
	}
	r.setAllocToGlobalCache(alloc)
	return alloc, nil
}

func (r *Repository) GetByIdAndLock(ctx context.Context, id string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	cache, err := getCache(tx)
	if err != nil {
		return nil, err
	}

	alloc := &Allocation{}
	err = tx.Model(&Allocation{}).
		Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Where("id=?", id).
		Take(alloc).Error
	if err != nil {
		return alloc, err
	}
	cache[id] = AllocationCache{
		Allocation: alloc,
	}
	r.setAllocToGlobalCache(alloc)
	return alloc, err
}

func (r *Repository) GetByTx(ctx context.Context, allocationID, txHash string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	cache, err := getCache(tx)
	if err != nil {
		return nil, err
	}
	if a, ok := cache[allocationID]; ok {
		if a.Allocation.Tx == txHash {
			return a.Allocation, nil
		}
	}

	a := r.getAllocFromGlobalCache(allocationID)
	if a != nil && a.Tx == txHash {
		cache[allocationID] = AllocationCache{
			Allocation: a,
		}
		return a, nil
	}

	alloc := &Allocation{}
	err = tx.Table(TableNameAllocation).Where(SQLWhereGetByTx, txHash).Take(alloc).Error
	if err != nil {
		return alloc, err
	}
	cache[allocationID] = AllocationCache{
		Allocation: alloc,
	}
	r.setAllocToGlobalCache(alloc)
	return alloc, err
}

func (r *Repository) GetAllocations(ctx context.Context, offset int64) ([]*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)

	const query = `finalized = false AND cleaned_up = false`
	allocs := make([]*Allocation, 0, 10)
	err := tx.Model(&Allocation{}).
		Where(query).
		Limit(UPDATE_LIMIT).
		Offset(int(offset)).
		Order("id ASC").
		Find(&allocs).Error
	if err != nil {
		return allocs, err
	}
	for ind, alloc := range allocs {
		if ind == lruSize {
			break
		}
		r.setAllocToGlobalCache(alloc)
	}
	return allocs, nil
}

func (r *Repository) GetAllocationFromDB(ctx context.Context, allocationID string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)

	alloc := &Allocation{}
	err := tx.Model(&Allocation{}).Where("id = ?", allocationID).Take(alloc).Error
	if err != nil {
		return nil, err
	}
	return alloc, nil
}

func (r *Repository) GetAllocationIds(ctx context.Context) []Res {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	var res []Res

	err := tx.Model(&Allocation{}).Select("id").Find(&res).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		logging.Logger.Error("error_getting_allocations_worker",
			zap.Any("error", err))
	}

	return res

}

func (r *Repository) UpdateAllocationRedeem(ctx context.Context, allocationID, AllocationRoot string, allocationObj *Allocation) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	cache, err := getCache(tx)
	if err != nil {
		return err
	}

	allocationUpdates := make(map[string]interface{})
	allocationUpdates["latest_redeemed_write_marker"] = AllocationRoot
	allocationUpdates["is_redeem_required"] = false
	err = tx.Model(allocationObj).Updates(allocationUpdates).Error
	if err != nil {
		return err
	}
	allocationObj.LatestRedeemedWM = AllocationRoot
	allocationObj.IsRedeemRequired = false
	txnCache := cache[allocationID]
	txnCache.Allocation = allocationObj
	updateAlloc := func(a *Allocation) {
		a.LatestRedeemedWM = AllocationRoot
		a.IsRedeemRequired = false
	}
	txnCache.AllocationUpdates = append(txnCache.AllocationUpdates, updateAlloc)
	cache[allocationID] = txnCache
	return nil
}

func (r *Repository) UpdateAllocation(ctx context.Context, allocationObj *Allocation, updateMap map[string]interface{}, updateOption AllocationUpdate) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	cache, err := getCache(tx)
	if err != nil {
		return err
	}
	err = tx.Model(allocationObj).Updates(updateMap).Error
	if err != nil {
		return err
	}
	txnCache := cache[allocationObj.ID]
	txnCache.Allocation = allocationObj
	txnCache.AllocationUpdates = append(txnCache.AllocationUpdates, updateOption)
	cache[allocationObj.ID] = txnCache
	return nil
}

func (r *Repository) Commit(tx *datastore.EnhancedDB) {
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	cache, _ := getCache(tx)
	if cache == nil {
		return
	}
	for _, txnCache := range cache {
		alloc := r.getAllocFromGlobalCache(txnCache.Allocation.ID)
		mapLock.Lock()
		mut, ok := r.allocLock[txnCache.Allocation.ID]
		if !ok {
			mut = &sync.Mutex{}
			r.allocLock[txnCache.Allocation.ID] = mut
		}
		mapLock.Unlock()
		mut.Lock()
		if alloc != nil {
			for _, update := range txnCache.AllocationUpdates {
				update(alloc)
			}
			r.setAllocToGlobalCache(alloc)
		}
		mut.Unlock()
	}
}

func (r *Repository) Save(ctx context.Context, alloc *Allocation) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}

	cache, err := getCache(tx)
	if err != nil {
		return err
	}

	txnCache := cache[alloc.ID]
	txnCache.Allocation = alloc
	err = tx.Save(alloc).Error
	if err != nil {
		return err
	}
	updateAlloc := func(a *Allocation) {
		*a = *alloc
	}
	txnCache.AllocationUpdates = append(txnCache.AllocationUpdates, updateAlloc)
	cache[alloc.ID] = txnCache
	return nil
}

func (r *Repository) Create(ctx context.Context, alloc *Allocation) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	cache, err := getCache(tx)
	if err != nil {
		return err
	}

	txnCache := cache[alloc.ID]
	txnCache.Allocation = alloc
	err = tx.Create(alloc).Error
	if err != nil {
		return err
	}
	cache[alloc.ID] = txnCache
	return nil
}

func getCache(tx *datastore.EnhancedDB) (map[string]AllocationCache, error) {
	c, ok := tx.SessionCache[TableNameAllocation]
	if ok {
		cache, ok := c.(map[string]AllocationCache)
		if !ok {
			return nil, fmt.Errorf("type assertion failed")
		}
		return cache, nil
	}
	cache := make(map[string]AllocationCache)
	tx.SessionCache[TableNameAllocation] = cache
	if tx.CommitAllocCache == nil {
		tx.CommitAllocCache = func(tx *datastore.EnhancedDB) {
			Repo.Commit(tx)
		}
	}
	return cache, nil
}

func (r *Repository) getAllocFromGlobalCache(id string) *Allocation {
	a, ok := r.allocCache.Get(id)
	if !ok {
		return nil
	}
	return &a
}

func (r *Repository) setAllocToGlobalCache(a *Allocation) {
	if a != nil {
		r.allocCache.Add(a.ID, *a)
	}
}

func (r *Repository) DeleteAllocation(allocationID string) {
	r.allocCache.Remove(allocationID)
}
