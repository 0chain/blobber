package allocation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"0chain.net/blobbercore/datastore"
	"0chain.net/blobbercore/reference"
	"0chain.net/core/chain"
	"0chain.net/core/common"
	"0chain.net/core/lock"
	"0chain.net/core/transaction"

	. "0chain.net/core/logging"

	"github.com/jinzhu/gorm"
	"go.uber.org/zap"
)

const (
	UPDATE_LIMIT       = 100             // items
	UPDATE_DB_INTERVAL = 5 * time.Second //
	REQUEST_TIMEOUT    = 1 * time.Second //
)

func StartUpdateWorker(ctx context.Context, interval time.Duration) {
	go UpdateWorker(ctx, interval)
}

// UpdateWorker updates all not finalized and not cleaned allocations
// requesting SC through REST API. The worker required to fetch allocations
// updates in DB.
func UpdateWorker(ctx context.Context, interval time.Duration) {
	Logger.Info("start update allocations worker")

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

	var (
		allocs []*Allocation
		count  int
		offset int

		err error
	)

	// iterate all in loop accepting allocations with limit

	for start := true; start || (offset < count); start = false {

		println("updateWork", offset, count)

		allocs, count, err = findAllocations(ctx, offset)
		if err != nil {
			Logger.Error("finding allocations in DB", zap.Error(err))
			if waitOrQuit(ctx, UPDATE_DB_INTERVAL) {
				return
			}
			continue
		}

		println("updateWork", "A", len(allocs), "O", offset, "C", count)
		offset += len(allocs)
		println("updateWork", "A", len(allocs), "O", offset, "C", count)

		for _, a := range allocs {
			updateAllocation(ctx, a)
			if waitOrQuit(ctx, REQUEST_TIMEOUT) {
				return
			}
		}
	}
}

// not finalized, not cleaned up
func findAllocations(ctx context.Context, offset int) (
	allocs []*Allocation, count int, err error) {

	println("findAllocations", offset)

	const query = `finalized = false AND cleaned_up = false`

	ctx = datastore.GetStore().CreateTransaction(ctx)

	var tx = datastore.GetStore().GetTransaction(ctx)
	defer tx.Rollback()

	err = tx.Model(&Allocation{}).Where(query).Count(&count).Error
	if err != nil {
		return
	}

	allocs = make([]*Allocation, 0) // have to make for the GROM (stupid GORM)
	err = tx.Model(&Allocation{}).
		Where(query).
		Limit(UPDATE_LIMIT).
		Offset(offset).
		Order("id ASC").
		Find(&allocs).Error
	return
}

func shouldFinalize(sa *transaction.StorageAllocation) bool {
	var now = common.Now()
	return sa.Until() < now && !sa.Finalized
}

func updateAllocation(ctx context.Context, a *Allocation) {

	println("updateAllocation", a.ID)

	if a.Finalized {
		cleanupAllocation(ctx, a)
		return
	}

	var sa, err = requestAllocation(a.ID)
	if err != nil {
		Logger.Error("requesting allocations from SC", zap.Error(err))
		println("updateAllocation", a.ID, "R 1")
		return
	}

	// if new Tx, then we have to update the allocation
	if sa.Tx != a.Tx {
		if a, err = updateAllocationInDB(ctx, a, sa); err != nil {
			Logger.Error("updating allocation in DB", zap.Error(err))
			println("updateAllocation", a.ID, "R 2")
			return
		}
	}

	println("SA F:", sa.Finalized, "SF", shouldFinalize(sa))
	println("A  F:", a.Finalized, "CU", a.CleanedUp)

	// send finalize allocation transaction
	if shouldFinalize(sa) {
		sendFinalizeAllocation(a)
		println("updateAllocation", a.ID, "R 3")
		return
	}

	// remove data
	if a.Finalized && !a.CleanedUp {
		println("updateAllocation", a.ID, "R 4")
		cleanupAllocation(ctx, a)
	}

	println("updateAllocation", a.ID, "R 5", "F", a.Finalized, "CU", a.CleanedUp)
}

func requestAllocation(allocID string) (
	sa *transaction.StorageAllocation, err error) {

	println("requestAllocation", allocID)

	var b []byte
	b, err = transaction.MakeSCRestAPICall(
		transaction.STORAGE_CONTRACT_ADDRESS,
		"/allocation",
		map[string]string{"allocation": allocID},
		chain.GetServerChain(),
		nil)
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

func updateAllocationInDB(ctx context.Context, a *Allocation,
	sa *transaction.StorageAllocation) (ua *Allocation, err error) {

	println("updateAllocationInDB", a.ID)

	ctx = datastore.GetStore().CreateTransaction(ctx)

	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	var changed bool

	changed = a.Tx != sa.Tx

	// transaction
	a.Tx = sa.Tx

	// update fields
	a.Expiration = sa.Expiration
	a.TotalSize = sa.Size
	a.Finalized = sa.Finalized

	// update terms
	a.Terms = make([]*Terms, 0, len(sa.BlobberDetails))
	for _, d := range sa.BlobberDetails {
		a.Terms = append(a.Terms, &Terms{
			BlobberID:    d.BlobberID,
			AllocationTx: sa.Tx,
			ReadPrice:    d.Terms.ReadPrice,
			WritePrice:   d.Terms.WritePrice,
		})
	}

	// save allocations
	if err = tx.Save(a).Error; err != nil {
		return nil, err
	}

	if !changed {
		return a, nil
	}

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

	return a, nil // ok
}

type finalizeRequest struct {
	AllocationID string `json:"allocation_id"`
}

func (fr *finalizeRequest) marshal() string {
	var b, err = json.Marshal(fr)
	if err != nil {
		panic(err) // must never happens
	}
	return string(b)
}

func sendFinalizeAllocation(a *Allocation) {
	println("sendFinalizeAllocation", a.ID)

	var tx, err = transaction.NewTransactionEntity()
	if err != nil {
		Logger.Error("creating new transaction entity", zap.Error(err))
		return
	}

	var request finalizeRequest
	request.AllocationID = a.ID

	err = tx.ExecuteSmartContract(
		transaction.STORAGE_CONTRACT_ADDRESS,
		transaction.FINALIZE_ALLOCATION,
		request.marshal(),
		0)
	if err != nil {
		Logger.Error("sending finalize allocation", zap.Error(err))
		return
	}
}

func cleanupAllocation(ctx context.Context, a *Allocation) {
	println("cleanupAllocation", a.ID)

	var err error
	if err = deleteInFakeConnection(ctx, a); err != nil {
		Logger.Error("cleaning finalized allocation", zap.Error(err))
	}

	ctx = datastore.GetStore().CreateTransaction(ctx)
	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	a.CleanedUp = true
	if err = tx.Update(a).Error; err != nil {
		Logger.Error("updating allocation 'cleaned_up'", zap.Error(err))
	}
}

func newConnectionID() string {
	var nBig, err = rand.Int(rand.Reader, big.NewInt(0xffffffff))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d", nBig.Int64())
}

func deleteInFakeConnection(ctx context.Context, a *Allocation) (err error) {
	var (
		connID = newConnectionID()
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
func deleteFiles(ctx context.Context, allocID string,
	conn *AllocationChangeCollector) (err error) {

	println("DELTE FILES:", allocID)

	ctx = datastore.GetStore().CreateTransaction(ctx)

	var tx = datastore.GetStore().GetTransaction(ctx)
	defer commit(tx, &err)

	var refs = make([]*reference.Ref, 0)
	err = tx.Where(&reference.Ref{
		Type:         reference.FILE,
		AllocationID: allocID,
	}).Find(&refs).Error
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		return
	}
	err = nil // reset the record not found error

	for _, ref := range refs {
		println("FOUND (DELETE):", ref.Path)
		if err = deleteFile(ctx, ref.Path, conn); err != nil {
			return
		}
	}

	return
}

// delete reference
func deleteFile(ctx context.Context, path string,
	conn *AllocationChangeCollector) (err error) {

	println("DELTE FILE:", path)

	var fileRef *reference.Ref
	fileRef, err = reference.GetReference(ctx, conn.AllocationID, path)
	if err != nil {
		return
	}

	var (
		deleteSize = fileRef.Size
		change     = new(AllocationChange)
	)

	change.ConnectionID = conn.ConnectionID
	change.Size = 0 - deleteSize
	change.Operation = DELETE_OPERATION

	var dfc = &DeleteFileChange{
		ConnectionID: conn.ConnectionID,
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
