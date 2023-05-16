package allocation

import (
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SyncAllocation try to pull allocation from blockchain, and insert it in db.
// Check if the allocation already exists in the db by allocation ID.
// If exists, update the allocation with the latest data from blockchain but do not overwrite the allocation root.
// If not exists, create a new allocation in db.
func SyncAllocation(allocationTx string) (*Allocation, error) {

	logging.Logger.Info("SyncAllocation jayash", zap.Any("tx", allocationTx))

	t, err := transaction.VerifyTransaction(allocationTx, chain.GetServerChain())
	if err != nil {
		return nil, errors.Throw(common.ErrBadRequest,
			"Invalid Allocation id. Allocation not found in blockchain.")
	}

	logging.Logger.Info("jayash TX output", zap.Any("tOutput", t.TransactionOutput))

	var sa transaction.StorageAllocation
	err = json.Unmarshal([]byte(t.TransactionOutput), &sa)
	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrInternal, "Error decoding the edbAllocation transaction output.")
	}

	logging.Logger.Info("jayash SA", zap.Any("SA", sa))

	db := datastore.GetStore().GetDB()
	a := new(Allocation)

	var isExist bool
	err = db.Model(&Allocation{}).
		Where("id = ?", sa.ID).
		First(a).Error
	logging.Logger.Error("jayash special edbAllocation", zap.Any("a", a), zap.Any("err", err))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, common.NewError("bad_db_operation", err.Error()) // unexpected
	}

	isExist = a.ID != ""

	logging.Logger.Info("jayash isExist", zap.Any("isExist", isExist))

	edbAllocation, _ := requestAllocation(sa.ID)

	logging.Logger.Info("jayash Allocation", zap.Any("edbAllocation", edbAllocation))

	alloc := &Allocation{}

	if !isExist {
		belongToThisBlobber := false
		for _, blobberConnection := range sa.BlobberDetails {
			if blobberConnection.BlobberID == node.Self.ID {
				belongToThisBlobber = true

				alloc.AllocationRoot = ""
				alloc.BlobberSize = (edbAllocation.Size + edbAllocation.DataShards - 1) /
					edbAllocation.DataShards
				alloc.BlobberSizeUsed = 0

				break
			}
		}
		if !belongToThisBlobber {
			return nil, errors.Throw(common.ErrBadRequest,
				"Blobber is not part of the open connection transaction")
		}
	}

	// set/update fields
	alloc.ID = edbAllocation.ID
	alloc.Tx = edbAllocation.Tx
	alloc.Expiration = edbAllocation.Expiration
	alloc.OwnerID = edbAllocation.OwnerID
	alloc.OwnerPublicKey = edbAllocation.OwnerPublicKey
	alloc.RepairerID = t.ClientID // blobber node id
	alloc.TotalSize = edbAllocation.Size
	alloc.UsedSize = edbAllocation.UsedSize
	alloc.Finalized = edbAllocation.Finalized
	alloc.TimeUnit = edbAllocation.TimeUnit
	alloc.FileOptions = edbAllocation.FileOptions
	alloc.BlobberSize = (edbAllocation.Size + edbAllocation.DataShards - 1) / edbAllocation.DataShards

	m := map[string]interface{}{
		"allocation_id":  alloc.ID,
		"allocated_size": alloc.BlobberSize,
	}

	err = filestore.GetFileStore().UpdateAllocationMetaData(m)
	if err != nil {
		return nil, common.NewError("meta_data_update_error", err.Error())
	}

	// related terms
	terms := make([]*Terms, 0, len(sa.BlobberDetails))
	for _, d := range sa.BlobberDetails {
		terms = append(terms, &Terms{
			BlobberID:    d.BlobberID,
			AllocationID: alloc.ID,
			ReadPrice:    d.Terms.ReadPrice,
			WritePrice:   d.Terms.WritePrice,
		})
	}

	logging.Logger.Info("jayash onlyAlloc", zap.Any("alloc", alloc))

	// check if edbAllocation exists by id in db and update it or create new one
	if isExist {
		err = db.Transaction(func(tx *gorm.DB) error {
			if err = tx.Model(&Allocation{}).
				Where("id = ?", alloc.ID).
				Updates(alloc).Error; err != nil {
				logging.Logger.Info("jayash error1", zap.Any("err", err))
				return err
			}

			if err = tx.Model(&Terms{}).
				Where("allocation_id = ?", alloc.ID).
				Delete(&Terms{}).Error; err != nil {
				logging.Logger.Info("jayash error2", zap.Any("err", err))
				return err
			}

			if err = tx.Model(&Terms{}).
				Create(terms).Error; err != nil {
				logging.Logger.Info("jayash error3", zap.Any("err", err))
				return err
			}

			return nil
		})
	} else {
		err = db.Transaction(func(tx *gorm.DB) error {
			if err = tx.Create(alloc).Error; err != nil {
				logging.Logger.Info("jayash error4", zap.Any("err", err))
				return err
			}

			if err = tx.Create(terms).Error; err != nil {
				logging.Logger.Info("jayash error5", zap.Any("err", err))
				return err
			}

			return nil
		})
	}

	return alloc, err
}
