package allocation

import (
	"context"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	SQLWhereGetById = "allocations.id = ?"
	SQLWhereGetByTx = "allocations.tx = ?"
)

var (
	Repo *Repository
)

func init() {
	Repo = &Repository{}
}

type Repository struct {
}

type Res struct {
	ID string
}

func (r *Repository) GetById(ctx context.Context, id string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	var cache map[string]*Allocation
	c, ok := tx.SessionCache[TableNameAllocation]
	if !ok {
		cache = make(map[string]*Allocation)
		tx.SessionCache[TableNameAllocation] = cache
	}
	cache = c.(map[string]*Allocation)
	if a, ok := cache[id]; ok {
		return a, nil
	}

	alloc := &Allocation{}
	err := tx.Table(TableNameAllocation).Where(SQLWhereGetById, id).First(alloc).Error
	if err != nil {
		return alloc, err
	}

	cache[id] = alloc

	return alloc, err
}

func (r *Repository) GetByIdAndLock(ctx context.Context, id string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	var cache map[string]*Allocation
	c, ok := tx.SessionCache[TableNameAllocation]
	if !ok {
		cache = make(map[string]*Allocation)
		tx.SessionCache[TableNameAllocation] = cache
	}
	cache = c.(map[string]*Allocation)

	alloc := &Allocation{}

	err := tx.Model(&Allocation{}).
		Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).
		Where("id=?", id).
		First(alloc).Error
	if err != nil {
		return alloc, err
	}
	cache[id] = alloc

	return alloc, err
}

func (r *Repository) GetByTx(ctx context.Context, allocationID, txHash string) (*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	var cache map[string]*Allocation
	c, ok := tx.SessionCache[TableNameAllocation]
	if !ok {
		c = make(map[string]*Allocation)
		tx.SessionCache[TableNameAllocation] = c
	}
	cache = c.(map[string]*Allocation)
	if a, ok := cache[allocationID]; ok {
		if a.Tx == txHash {
			return a, nil
		}
	}

	alloc := &Allocation{}
	err := tx.Table(TableNameAllocation).Where(SQLWhereGetByTx, txHash).First(alloc).Error
	if err != nil {
		return alloc, err
	}
	cache[allocationID] = alloc

	return alloc, err
}

func (r *Repository) GetAllocations(ctx context.Context, offset int64) ([]*Allocation, error) {
	var tx = datastore.GetStore().GetTransaction(ctx)

	const query = `finalized = false AND cleaned_up = false`
	allocs := make([]*Allocation, 0)
	return allocs, tx.Model(&Allocation{}).
		Where(query).
		Limit(UPDATE_LIMIT).
		Offset(int(offset)).
		Order("id ASC").
		Find(&allocs).Error
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

func (r *Repository) UpdateAllocationRedeem(ctx context.Context, allocationID, AllocationRoot string) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	var cache map[string]*Allocation
	c, ok := tx.SessionCache[TableNameAllocation]
	if !ok {
		c = make(map[string]*Allocation)
		tx.SessionCache[TableNameAllocation] = c
	}
	cache = c.(map[string]*Allocation)
	delete(cache, allocationID)

	err := tx.Exec("UPDATE allocations SET latest_redeemed_write_marker=?,is_redeem_required=? WHERE id=?",
		AllocationRoot, false, allocationID).Error

	return err
}

func (r *Repository) Save(ctx context.Context, a *Allocation) error {
	var tx = datastore.GetStore().GetTransaction(ctx)
	if tx == nil {
		logging.Logger.Panic("no transaction in the context")
	}
	var cache map[string]*Allocation
	c, ok := tx.SessionCache[TableNameAllocation]
	if !ok {
		c = make(map[string]*Allocation)
		tx.SessionCache[TableNameAllocation] = c
	}
	cache = c.(map[string]*Allocation)

	cache[a.ID] = a
	return tx.Save(a).Error
}
