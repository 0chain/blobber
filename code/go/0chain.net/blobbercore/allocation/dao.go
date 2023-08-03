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

	db := store.CreateTransaction(ctx)

	if len(allocationId) == 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "tx")
	}

	alloc, err := Repo.GetById(db, allocationId)
	tx := store.GetTransaction(ctx)
	tx.Rollback()

	if err == nil {
		return alloc, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {

		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	return SyncAllocation(allocationId)

}

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
