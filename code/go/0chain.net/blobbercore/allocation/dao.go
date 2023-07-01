package allocation

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"
)

// GetOrCreate, get allocation if it exists in db. if not, try to sync it from blockchain, and insert it in db.
func GetOrCreate(ctx context.Context, store datastore.Store, allocationId string) (*Allocation, error) {

	db := store.GetDB()

	if len(allocationId) == 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "tx")
	}

	cachedAllocationInterface, err := LRU.Get(allocationId)
	if err == nil {
		cachedAllocation := cachedAllocationInterface.(*Allocation)
		return cachedAllocation, nil
	}

	alloc := &Allocation{}
	result := db.Table(TableNameAllocation).Where(SQLWhereGetById, allocationId).First(alloc)

	if result.Error == nil {
		return alloc, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {

		return nil, errors.ThrowLog(result.Error.Error(), common.ErrBadDataStore)
	}

	return SyncAllocation(allocationId)

}

const (
	SQLWhereGetByTx = "allocations.tx = ?"
	SQLWhereGetById = "allocations.id = ?"
)

// DryRun  Creates a prepared statement when executing any SQL and caches them to speed up future calls
// https://gorm.io/docs/performance.html#Caches-Prepared-Statement
func DryRun(db *gorm.DB) {

	// https://gorm.io/docs/session.html#DryRun
	// Session mode
	tx := db.Session(&gorm.Session{PrepareStmt: true, DryRun: true})

	// use Table instead of Model to reduce reflect times

	// prepare statement for GetOrCreate
	tx.Table(TableNameAllocation).Where(SQLWhereGetByTx, "tx").First(&Allocation{})

}
