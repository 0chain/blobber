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
func GetOrCreate(ctx context.Context, allocationId string) (*Allocation, error) {

	db := datastore.GetStore().CreateTransaction(ctx)
	// tx := datastore.GetStore().GetTransaction(ctx)

	if len(allocationId) == 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "tx")
	}

	alloc, err := Repo.GetById(db, allocationId)
	// tx.Rollback()

	if err == nil {
		return alloc, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {

		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	return SyncAllocation(allocationId)

}
