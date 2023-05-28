package allocation

import (
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/errors"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SyncAllocation try to pull allocation from blockchain, and insert it in db.
func SyncAllocation(allocationId string) (*Allocation, error) {
	logging.Logger.Info("jayash SyncAllocation", zap.String("allocationId", allocationId))

	sa, err := requestAllocation(allocationId)
	if err != nil {
		return nil, err
	}

	logging.Logger.Info("jayash SyncAllocation 2", zap.Any("sa", sa))

	alloc := &Allocation{}

	belongToThisBlobber := false
	for _, blobberConnection := range sa.BlobberDetails {
		if blobberConnection.BlobberID == node.Self.ID {
			belongToThisBlobber = true

			alloc.AllocationRoot = ""
			alloc.BlobberSize = (sa.Size + int64(len(sa.BlobberDetails)-1)) /
				int64(len(sa.BlobberDetails))
			alloc.BlobberSizeUsed = 0

			break
		}
	}

	logging.Logger.Info("jayash SyncAllocation 3", zap.Any("belongToThisBlobber", belongToThisBlobber))

	if !belongToThisBlobber {
		return nil, errors.Throw(common.ErrBadRequest,
			"Blobber is not part of the open connection transaction")
	}

	// set/update fields
	alloc.ID = sa.ID
	alloc.Tx = sa.Tx
	alloc.Expiration = sa.Expiration
	alloc.OwnerID = sa.OwnerID
	alloc.OwnerPublicKey = sa.OwnerPublicKey
	alloc.RepairerID = node.Self.ID // blobber node id
	alloc.TotalSize = sa.Size
	alloc.UsedSize = sa.UsedSize
	alloc.Finalized = sa.Finalized
	alloc.TimeUnit = sa.TimeUnit
	alloc.FileOptions = sa.FileOptions

	logging.Logger.Info("jayash SyncAllocation 4", zap.Any("alloc", alloc))

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

	logging.Logger.Info("jayash SyncAllocation 5", zap.Any("terms", terms))

	err = datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(TableNameAllocation).Create(alloc).Error; err != nil {
			return err
		}

		for _, term := range terms {
			if err := tx.Table(TableNameTerms).Create(term).Error; err != nil {
				return err
			}
		}

		return nil
	})

	logging.Logger.Info("jayash SyncAllocation 6", zap.Any("err", err))

	return alloc, err
}
