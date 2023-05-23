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

// SyncAllocation try to pull allocation using allocation ID from blockchain, and insert it in db.
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

	// check if sa.ID is already in db
	db := datastore.GetStore().GetDB()
	alloc := &Allocation{}
	result := db.Table(TableNameAllocation).Where("allocations.id = ?", sa.ID).First(alloc)

	isExists := false

	if result.Error == nil && alloc.ID == sa.ID {
		isExists = true
	}

	alloc = &Allocation{}

	if !isExists {
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
		if !belongToThisBlobber {
			return nil, errors.Throw(common.ErrBadRequest,
				"Blobber is not part of the open connection transaction")
		}
	}

	// set/update fields
	alloc.ID = sa.ID
	alloc.Tx = sa.Tx
	alloc.Expiration = sa.Expiration
	alloc.OwnerID = sa.OwnerID
	alloc.OwnerPublicKey = sa.OwnerPublicKey
	alloc.RepairerID = t.ClientID // blobber node id
	alloc.TotalSize = sa.Size
	if !isExists {
		alloc.UsedSize = sa.UsedSize
	}
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

	err = datastore.GetStore().GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(TableNameAllocation).Save(alloc).Error; err != nil {
			return err
		}

		for _, term := range terms {
			if err := tx.Table(TableNameTerms).Save(term).Error; err != nil {
				return err
			}
		}

		return nil
	})

	return alloc, err
}
