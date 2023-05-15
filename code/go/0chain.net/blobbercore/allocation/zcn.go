package allocation

import (
	"encoding/json"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/node"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/errors"
	"gorm.io/gorm"
)

// SyncAllocation try to pull allocation from blockchain, and insert it in db.
func SyncAllocation(allocationTx string) (*Allocation, error) {
	t, err := transaction.VerifyTransaction(allocationTx, chain.GetServerChain())
	if err != nil {
		return nil, errors.Throw(common.ErrBadRequest,
			"Invalid Allocation id. Allocation not found in blockchain.")
	}
	var sa transaction.StorageAllocation
	err = json.Unmarshal([]byte(t.TransactionOutput), &sa)
	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrInternal, "Error decoding the allocation transaction output.")
	}

	allocation, _ := requestAllocation(sa.ID)

	alloc := &Allocation{}

	belongToThisBlobber := false
	for _, blobberConnection := range sa.BlobberDetails {
		if blobberConnection.BlobberID == node.Self.ID {
			belongToThisBlobber = true

			alloc.AllocationRoot = ""
			alloc.BlobberSize = (allocation.Size + allocation.DataShards - 1) /
				allocation.DataShards
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
	alloc.RepairerID = t.ClientID // blobber node id
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

	// check if allocation exists by id in db and update it or create new one
	datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(TableNameAllocation).Where(Allocation{ID: alloc.ID}).Assign(alloc).FirstOrCreate(alloc).Error; err != nil {
			return err
		}

		for _, term := range terms {
			if err := tx.Table(TableNameTerms).FirstOrCreate(term, term).Error; err != nil {
				return err
			}
		}

		return nil
	})

	return alloc, err
}
