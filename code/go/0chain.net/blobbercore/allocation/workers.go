package allocation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/reference"
	"github.com/0chain/blobber/code/go/0chain.net/core/chain"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/lock"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/blobber/code/go/0chain.net/core/transaction"
	"github.com/0chain/gosdk/constants"
	"github.com/0chain/gosdk/zboxcore/zboxutil"

	"gorm.io/gorm"

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
			updateWork(ctx)
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
		count  int64
		offset int64

		err error
	)

	// iterate all in loop accepting allocations with limit

	for start := true; start || (offset < count); start = false {
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
			updateAllocation(ctx, a)
			if waitOrQuit(ctx, REQUEST_TIMEOUT) {
				return
			}
		}
	}
}

// not finalized, not cleaned up
func findAllocations(ctx context.Context, offset int64) (allocs []*Allocation, count int64, err error) {
	const query = `finalized = false AND cleaned_up = false`

	ctx = datastore.GetStore().CreateTransaction(ctx)

	var tx = datastore.GetStore().GetTransaction(ctx)
	defer tx.Rollback()

	err = tx.Model(&Allocation{}).Where(query).Count(&count).Error
	if err != nil {
		logging.Logger.Error(err.Error())
		return
	}

	allocs = make([]*Allocation, 0)
	err = tx.Model(&Allocation{}).
		Where(query).
		Limit(UPDATE_LIMIT).
		Offset(int(offset)).
		Order("id ASC").
		Find(&allocs).Error
	return
}

func shouldFinalize(sa *transaction.StorageAllocation) bool {
	var now = common.Now()
	return sa.Until() < now && !sa.Finalized
}

func updateAllocation(ctx context.Context, a *Allocation) {
	if a.Finalized {
		cleanupAllocation(ctx, a)
		return
	}

	var sa, err = requestAllocation(a.ID)
	if err != nil {
		logging.Logger.Error("requesting allocations from SC", zap.Error(err))
		return
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
		sendFinalizeAllocation(a)
		return
	}

	// remove data
	if a.Finalized && !a.CleanedUp {
		cleanupAllocation(ctx, a)
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

func commit(tx *gorm.DB, err *error) {
	if (*err) != nil {
		tx.Rollback()
		return
	}
	(*err) = tx.Commit().Error
}

func updateAllocationInDB(ctx context.Context, a *Allocation, sa *transaction.StorageAllocation) (ua *Allocation, err error) {
	ctx = datastore.GetStore().CreateTransaction(ctx)

	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	var changed bool = a.Tx != sa.Tx

	// transaction
	a.Tx = sa.Tx
	a.OwnerID = sa.OwnerID
	a.OwnerPublicKey = sa.OwnerPublicKey

	// update fields
	a.Expiration = sa.Expiration
	a.TotalSize = sa.Size
	a.Finalized = sa.Finalized
	a.FileOptions = sa.FileOptions
	a.BlobberSize = sa.Size / sa.DataShards

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
	if err := tx.Save(a).Error; err != nil {
		return nil, err
	}

	if !changed {
		return a, nil
	}

	// save allocation terms
	for _, t := range a.Terms {
		if err := tx.Save(t).Error; err != nil {
			return nil, err
		}
	}

	return a, nil // ok
}

type finalizeRequest struct {
	AllocationID string `json:"allocation_id"`
}

func sendFinalizeAllocation(a *Allocation) {
	var tx, err = transaction.NewTransactionEntity()
	if err != nil {
		logging.Logger.Error("creating new transaction entity", zap.Error(err))
		return
	}

	var request finalizeRequest
	request.AllocationID = a.ID

	// TODO should this be verified?
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
	if err = deleteInFakeConnection(ctx, a); err != nil {
		logging.Logger.Error("cleaning finalized allocation", zap.Error(err))
	}

	ctx = datastore.GetStore().CreateTransaction(ctx)
	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	a.CleanedUp = true
	if err = tx.Model(a).Updates(a).Error; err != nil {
		logging.Logger.Error("updating allocation 'cleaned_up'", zap.Error(err))
	}
}

func deleteInFakeConnection(ctx context.Context, a *Allocation) (err error) {
	ctx = datastore.GetStore().CreateTransaction(ctx)
	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	var (
		connID = zboxutil.NewConnectionId()
		conn   *AllocationChangeCollector
	)
	conn, err = GetAllocationChanges(ctx, connID, a.ID, a.OwnerID)
	if err != nil {
		return
	}

	var mutex = lock.GetMutex(conn.TableName(), connID)
	mutex.Lock()
	defer mutex.Unlock()

	// list files, delete files
	if err = deleteFiles(ctx, a.ID, conn); err != nil {
		return
	}

	return conn.Save(ctx) // save the fake connection
}

// delete references
func deleteFiles(ctx context.Context, allocID string, conn *AllocationChangeCollector) (err error) {
	var (
		tx   = datastore.GetStore().GetTransaction(ctx)
		refs = make([]*reference.Ref, 0)
	)
	err = tx.Where(&reference.Ref{
		Type:         reference.FILE,
		AllocationID: allocID,
	}).Find(&refs).Error
	if err != nil {
		return
	}

	for _, ref := range refs {
		if err = deleteFile(ctx, ref.Path, conn); err != nil {
			return
		}
	}

	return
}

// delete reference
func deleteFile(ctx context.Context, path string, conn *AllocationChangeCollector) (err error) {
	var fileRef *reference.Ref
	fileRef, err = reference.GetLimitedRefFieldsByPath(ctx, conn.AllocationID, path, []string{"path", "name", "size", "hash", "fixed_merkle_root"})
	if err != nil {
		return
	}

	var (
		deleteSize = fileRef.Size
		change     = new(AllocationChange)
	)

	change.ConnectionID = conn.ID
	change.Size = 0 - deleteSize
	change.Operation = constants.FileOperationDelete

	var dfc = &DeleteFileChange{
		ConnectionID: conn.ID,
		AllocationID: conn.AllocationID,
		Name:         fileRef.Name,
		Hash:         fileRef.Hash,
		Path:         fileRef.Path,
		Size:         deleteSize,
	}

	conn.Size += change.Size
	conn.AddChange(change, dfc)
	return
}
