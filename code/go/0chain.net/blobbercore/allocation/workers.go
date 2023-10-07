package allocation

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/node"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"

	"go.uber.org/zap"
)

const (
	UPDATE_LIMIT       = 100             // items
	UPDATE_DB_INTERVAL = 5 * time.Second //
	REQUEST_TIMEOUT    = 1 * time.Second //
	REPAIR_TIMEOUT     = 900             // 15 Minutes
)

func StartUpdateWorker(ctx context.Context, interval time.Duration) {
	go UpdateWorker(ctx, interval)
}

func StartFinalizeWorker(ctx context.Context, interval time.Duration) {
	go FinalizeAllocationsWorker(ctx, interval)
}

// UpdateWorker updates all not finalized and not cleaned allocations
// requesting SC through REST API. The worker required to fetch allocations
// updates in DB.
func UpdateWorker(ctx context.Context, interval time.Duration) {
	logging.Logger.Info("start update allocations worker")

	var tk = time.NewTicker(interval)
	defer tk.Stop()

	var (
		tick = tk.C
		quit = ctx.Done()
	)

	for {
		select {
		case <-tick:
			updateCtx := datastore.GetStore().CreateTransaction(context.TODO())
			_ = datastore.GetStore().WithTransaction(updateCtx, func(ctx context.Context) error {
				updateWork(ctx)
				return nil
			})
			updateCtx.Done()
		case <-quit:
			return
		}
	}
}

func FinalizeAllocationsWorker(ctx context.Context, interval time.Duration) {
	logging.Logger.Info("start finalize allocations worker")

	var tk = time.NewTicker(interval)
	defer tk.Stop()

	var (
		tick = tk.C
		quit = ctx.Done()
	)

	for {
		select {
		case <-tick:
			_ = datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
				finalizeExpiredAllocations(ctx)
				return nil
			})
		case <-quit:
			return
		}
	}
}

func waitOrQuit(ctx context.Context, d time.Duration) (quit bool) {
	var tm = time.NewTimer(d)
	defer tm.Stop()

	var (
		elapsed = tm.C
		done    = ctx.Done()
	)

	select {
	case <-elapsed:
		return false // continue
	case <-done:
		return true // quit
	}
}

func updateWork(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logging.Logger.Error("[recover] updateWork", zap.Any("err", r))
		}
	}()

	var (
		allocs []*Allocation
		count  int
		offset int64

		err error
	)

	// iterate all in loop accepting allocations with limit

	for start := true; start || (offset < int64(count)); start = false {
		allocs, count, err = findAllocations(ctx, offset)
		if err != nil {
			logging.Logger.Error("finding allocations in DB", zap.Error(err))
			if waitOrQuit(ctx, UPDATE_DB_INTERVAL) {
				return
			}
			continue
		}

		offset += int64(len(allocs))

		for _, a := range allocs {
			updateAllocation(ctx, a, node.Self.ID)
			if waitOrQuit(ctx, REQUEST_TIMEOUT) {
				return
			}
		}
	}
}

// not finalized, not cleaned up
func findAllocations(ctx context.Context, offset int64) (allocs []*Allocation, count int, err error) {
	allocations, err := Repo.GetAllocations(ctx, offset)
	return allocations, len(allocations), err
}

func shouldFinalize(sa *transaction.StorageAllocation) bool {
	var now = common.Now()
	return sa.Expiration < now && !sa.Finalized
}

func updateAllocation(ctx context.Context, a *Allocation, selfBlobberID string) {
	if a.Finalized {
		cleanupAllocation(ctx, a)
		return
	}

	var sa, err = requestAllocation(a.ID)
	if err != nil {
		logging.Logger.Error("requesting allocations from SC", zap.Error(err))
		return
	}

	removedBlobber := true
	for _, d := range sa.BlobberDetails {
		if d.BlobberID == selfBlobberID {
			removedBlobber = false
			break
		}
	}

	if removedBlobber {
		logging.Logger.Info("blobber removed from allocation", zap.String("blobber", selfBlobberID), zap.String("allocation", a.ID))
		cleanupAllocation(ctx, a)
	}

	// if new Tx, then we have to update the allocation
	if sa.Tx != a.Tx || sa.OwnerID != a.OwnerID || sa.Finalized != a.Finalized {
		if a, err = updateAllocationInDB(ctx, a, sa); err != nil {
			logging.Logger.Error("updating allocation in DB", zap.Error(err))
			return
		}
	}

	// send finalize allocation transaction
	if shouldFinalize(sa) {
		sendFinalizeAllocation(a.ID)
		cleanupAllocation(ctx, a)
		return
	}

	// remove data
	if a.Finalized && !a.CleanedUp {
		cleanupAllocation(ctx, a)
	}
}

func finalizeExpiredAllocations(ctx context.Context) {
	var allocs, err = requestExpiredAllocations()
	if err != nil {
		logging.Logger.Error("requesting expired allocations from SC", zap.Error(err))
		return
	}

	for _, allocID := range allocs {
		sendFinalizeAllocation(allocID)
	}
}

func requestAllocation(allocID string) (sa *transaction.StorageAllocation, err error) {
	var b []byte
	b, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS,
		"/allocation",
		map[string]string{"allocation": allocID},
		chain.GetServerChain())
	if err != nil {
		return
	}
	sa = new(transaction.StorageAllocation)
	err = json.Unmarshal(b, sa)
	return
}

func requestExpiredAllocations() (allocs []string, err error) {
	var b []byte
	b, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS,
		"/expired-allocations",
		map[string]string{"blobber_id": node.Self.ID},
		chain.GetServerChain())
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &allocs)
	return
}

func updateAllocationInDB(ctx context.Context, a *Allocation, sa *transaction.StorageAllocation) (ua *Allocation, err error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	var changed bool = a.Tx != sa.Tx
	if !changed {
		return a, nil
	}

	// transaction
	a.Tx = sa.Tx
	a.OwnerID = sa.OwnerID
	a.OwnerPublicKey = sa.OwnerPublicKey

	// // update fields
	a.Expiration = sa.Expiration
	a.TotalSize = sa.Size
	a.Finalized = sa.Finalized
	a.FileOptions = sa.FileOptions
	a.BlobberSize = int64(math.Ceil(float64(sa.Size) / float64(sa.DataShards)))

	updateMap := make(map[string]interface{})
	updateMap["tx"] = a.Tx
	updateMap["owner_id"] = a.OwnerID
	updateMap["owner_public_key"] = a.OwnerPublicKey
	updateMap["expiration"] = a.Expiration
	updateMap["total_size"] = a.TotalSize
	updateMap["finalized"] = a.Finalized
	updateMap["file_options"] = a.FileOptions
	updateMap["blobber_size"] = a.BlobberSize

	updateOption := func(alloc *Allocation) {
		alloc.Tx = a.Tx
		alloc.OwnerID = a.OwnerID
		alloc.OwnerPublicKey = a.OwnerPublicKey
		alloc.Expiration = a.Expiration
		alloc.TotalSize = a.TotalSize
		alloc.Finalized = a.Finalized
		alloc.FileOptions = a.FileOptions
		alloc.BlobberSize = a.BlobberSize
	}

	// update terms
	a.Terms = make([]*Terms, 0, len(sa.BlobberDetails))
	for _, d := range sa.BlobberDetails {
		a.Terms = append(a.Terms, &Terms{
			BlobberID:    d.BlobberID,
			AllocationID: a.ID,
			ReadPrice:    d.Terms.ReadPrice,
			WritePrice:   d.Terms.WritePrice,
		})
	}

	// save allocations
	if err := Repo.UpdateAllocation(ctx, a, updateMap, updateOption); err != nil {
		return nil, err
	}

	// save allocation terms
	for _, t := range a.Terms {
		if err := tx.Save(t).Error; err != nil {
			return nil, err
		}
	}

	logging.Logger.Info("allocation updated", zap.String("id", a.ID), zap.Any("a", a))
	return a, nil // ok
}

type finalizeRequest struct {
	AllocationID string `json:"allocation_id"`
}

func sendFinalizeAllocation(allocationID string) {
	var tx, err = transaction.NewTransactionEntity()
	if err != nil {
		logging.Logger.Error("creating new transaction entity", zap.Error(err))
		return
	}

	var request finalizeRequest
	request.AllocationID = allocationID

	err = tx.ExecuteSmartContract(
		transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.FINALIZE_ALLOCATION,
		request,
		0)
	if err != nil {
		logging.Logger.Error("sending finalize allocation", zap.Error(err))
		return
	}
}

func cleanupAllocation(ctx context.Context, a *Allocation) {
	var err error
	if err = deleteAllocation(ctx, a); err != nil {
		logging.Logger.Error("cleaning finalized allocation", zap.Error(err))
	}

	var tx = datastore.GetStore().GetTransaction(ctx)

	a.CleanedUp = true
	if err = tx.Model(a).Updates(a).Error; err != nil {
		logging.Logger.Error("updating allocation 'cleaned_up'", zap.Error(err))
	}
}

func deleteAllocation(ctx context.Context, a *Allocation) (err error) {
	var tx = datastore.GetStore().GetTransaction(ctx)
	filestore.GetFileStore().DeleteAllocation(a.ID)
	err = tx.Model(&reference.Ref{}).Unscoped().
		Delete(&reference.Ref{},
			"allocation_id = ?",
			a.ID).Error
	return err
}
