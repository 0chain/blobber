package allocation

import (
	"context"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"gorm.io/gorm"
)

// GetOrCreate, get allocation if it exists in db. if not, try to sync it from blockchain, and insert it in db.
func GetOrCreate(ctx context.Context, store datastore.Store, allocationId string) (*Allocation, error) {
	logging.Logger.Info("jayash GetOrCreate", zap.String("allocationId", allocationId))

	return nil, nil

	db := store.GetDB()

	logging.Logger.Info("jayash GetOrCreate 2", zap.Any("db", db))

	if len(allocationId) == 0 {
		logging.Logger.Info("jayash GetOrCreate 3", zap.Any("allocationId", allocationId))
		return nil, errors.Throw(constants.ErrInvalidParameter, "tx")
	}

	alloc := &Allocation{}
	result := db.Table(TableNameAllocation).Where(SQLWhereGetById, allocationId).First(alloc)

	logging.Logger.Info("jayash GetOrCreate 4", zap.Any("result", result))

	if result.Error == nil {
		return alloc, nil
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		logging.Logger.Info("jayash GetOrCreate 5", zap.Any("result.Error", result.Error))

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
