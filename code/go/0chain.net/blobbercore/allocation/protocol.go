package allocation

import (
	"context"
	"encoding/json"
	"fmt"

	"0chain.net/blobbercore/datastore"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	. "0chain.net/core/logging"
	"0chain.net/core/node"
	"0chain.net/core/transaction"

	"github.com/jinzhu/gorm"
)

// GetAllocationByID from DB. This function doesn't load related terms.
func GetAllocationByID(ctx context.Context, allocID string) (
	a *Allocation, err error) {

	var tx = datastore.GetStore().GetTransaction(ctx)

	a = new(Allocation)
	err = tx.Model(&Allocation{}).
		Where(&Allocation{ID: allocID}).
		First(a).Error
	return
}

func VerifyAllocationTransaction(ctx context.Context, allocationTx string,
	readonly bool) (a *Allocation, err error) {

	var tx = datastore.GetStore().GetTransaction(ctx)

	a = new(Allocation)
	err = tx.Model(&Allocation{}).
		Where(&Allocation{Tx: allocationTx}).
		First(a).Error

	if err == nil {
		// load related terms
		var terms []*Terms
		err = tx.Model(terms).
			Where("allocation_tx = ?", a.Tx).
			Find(&terms).Error
		if err != nil && !gorm.IsRecordNotFoundError(err) {
			return // unexpected DB error
		}
		a.Terms = terms // set field
		err = nil       // reset the error
		return          // found in DB
	}

	if !gorm.IsRecordNotFoundError(err) {
		return nil, err // unexpected DB error
	}

	t, err := transaction.VerifyTransaction(allocationTx, chain.GetServerChain())
	if err != nil {
		return nil, common.NewError("invalid_allocation",
			"Invalid Allocation id. Allocation not found in blockchain. "+err.Error())
	}
	var sa transaction.StorageAllocation
	err = json.Unmarshal([]byte(t.TransactionOutput), &sa)
	if err != nil {
		return nil, common.NewError("transaction_output_decode_error",
			"Error decoding the allocation transaction output."+err.Error())
	}
	foundBlobber := false
	for _, blobberConnection := range sa.Blobbers {
		if blobberConnection.ID == node.Self.ID {
			foundBlobber = true
			a.AllocationRoot = ""
			a.BlobberSize = (sa.Size + int64(len(sa.Blobbers)-1)) /
				int64(len(sa.Blobbers))
			a.BlobberSizeUsed = 0
			break
		}
	}
	if !foundBlobber {
		return nil, common.NewError("invalid_blobber",
			"Blobber is not part of the open connection transaction")
	}
	a.ID = sa.ID
	a.Tx = sa.Tx
	a.Expiration = sa.Expiration
	a.OwnerID = sa.OwnerID
	a.OwnerPublicKey = sa.OwnerPublicKey
	a.TotalSize = sa.Size
	a.UsedSize = sa.UsedSize
	a.Finalized = sa.Finalized

	a.Terms = make([]*Terms, 0, len(sa.BlobberDetails))
	for _, d := range sa.BlobberDetails {
		a.Terms = append(a.Terms, &Terms{
			BlobberID:    d.BlobberID,
			AllocationTx: sa.Tx,
			ReadPrice:    d.Terms.ReadPrice,
			WritePrice:   d.Terms.WritePrice,
		})
	}

	if readonly {
		return
	}

	Logger.Info("Saving the allocation to DB")

	// save allocations
	var stub Allocation
	err = tx.Model(a).
		Where(&Allocation{Tx: sa.Tx}).
		Attrs(a).
		FirstOrCreate(&stub).Error
	if err != nil {
		return nil, err
	}

	// save or update client (the owner)

	// save allocation terms
	for _, t := range a.Terms {
		var stub Terms
		err = tx.Model(t).
			Where(Terms{BlobberID: t.BlobberID, AllocationTx: sa.Tx}).
			Assign(t).
			FirstOrCreate(&stub).Error
		if err != nil {
			return nil, err
		}
	}

	return
}

// read/write pool stat for an {allocation -> blobber}
type PoolStat struct {
	PoolID   string           `json:"pool_id"`
	Balance  int64            `json:"balance"`
	ExpireAt common.Timestamp `json:"expire_at"`
}

func RequestReadPools(clientID, allocationID string) (
	rps []*ReadPool, err error) {

	Logger.Info("request read pools")

	var (
		blobberID = node.Self.ID
		resp      []byte
	)

	resp, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS,
		"/getReadPoolAllocBlobberStat",
		map[string]string{
			"client_id":     clientID,
			"allocation_id": allocationID,
			"blobber_id":    blobberID,
		},
		chain.GetServerChain(), nil)
	if err != nil {
		return nil, fmt.Errorf("requesting read pools stat: %v", err)
	}

	var pss []*PoolStat
	if err = json.Unmarshal(resp, &pss); err != nil {
		return nil, fmt.Errorf("decoding read pools stat response: %v", err)
	}

	if len(pss) == 0 {
		return nil, nil // empty
	}

	rps = make([]*ReadPool, 0, len(pss))
	for _, ps := range pss {
		rps = append(rps, &ReadPool{
			PoolID: ps.PoolID,

			ClientID:     clientID,
			BlobberID:    blobberID,
			AllocationID: allocationID,

			Balance:  ps.Balance,
			ExpireAt: ps.ExpireAt,
		})
	}

	return // got them
}

func RequestWritePools(clientID, allocationID string) (
	wps []*WritePool, err error) {

	Logger.Info("request write pools")

	var (
		blobberID = node.Self.ID
		resp      []byte
	)

	resp, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS,
		"/getWritePoolAllocBlobberStat",
		map[string]string{
			"client_id":     clientID,
			"allocation_id": allocationID,
			"blobber_id":    blobberID,
		},
		chain.GetServerChain(), nil)
	if err != nil {
		return nil, fmt.Errorf("requesting write pools stat: %v", err)
	}

	var pss []*PoolStat
	if err = json.Unmarshal(resp, &pss); err != nil {
		return nil, fmt.Errorf("decoding write pools stat response: %v", err)
	}

	if len(pss) == 0 {
		return nil, nil // empty
	}

	wps = make([]*WritePool, 0, len(pss))
	for _, ps := range pss {
		wps = append(wps, &WritePool{
			PoolID: ps.PoolID,

			ClientID:     clientID,
			BlobberID:    blobberID,
			AllocationID: allocationID,

			Balance:  ps.Balance,
			ExpireAt: ps.ExpireAt,
		})
	}

	return // got them
}
