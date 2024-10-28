package allocation

import (
	"context"
	"math"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/errors"
	"go.uber.org/zap"
)

// SyncAllocation try to pull allocation from blockchain, and insert it in db.
func SyncAllocation(allocationId string) (*Allocation, error) {

	sa, err := requestAllocation(allocationId)
	if err != nil {
		return nil, err
	}

	alloc := &Allocation{}

	belongToThisBlobber := false
	for _, blobberConnection := range sa.BlobberDetails {
		if blobberConnection.BlobberID == node.Self.ID {
			belongToThisBlobber = true

			alloc.AllocationRoot = ""
			alloc.BlobberSize = int64(math.Ceil(float64(sa.Size) / float64(sa.DataShards)))
			alloc.BlobberSizeUsed = 0

			break
		}
	}

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

	err = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		var e error
		if e := Repo.Create(ctx, alloc); e != nil {
			return e
		}
		tx := datastore.GetStore().GetTransaction(ctx)
		for _, term := range terms {
			if err := tx.Table(TableNameTerms).Save(term).Error; err != nil {
				return e
			}
		}
		return e
	})

	if err != nil {
		return nil, errors.Throw(err, "meta_data_update_error", err.Error())
	}

	logging.Logger.Info("Saving the allocation to DB", zap.Any(
		"allocation", alloc), zap.Error(err))
	if err != nil {
		return nil, err
	}

	return alloc, err
}
