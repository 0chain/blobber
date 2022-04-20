package allocation

import (
	"errors"
	"fmt"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB

	CHUNK_SIZE = 64 * KB
)

// pendingMapLock lock read write access to pending table for specific client:allocationId combination.
// It contains separate lock for read and write pendings.
// eg: client1:alloc1:read --> lock for read pendings
// client1:alloc1:write --> lock for write pendings
// client1:alloc1 --> lock for writing read/write pendings
var pendingMapLock = common.GetLocker()

const (
	TableNameAllocation = "allocations"
)

type Allocation struct {
	ID             string           `gorm:"column:id;size:64;primaryKey"`
	Tx             string           `gorm:"column:tx;size:64;not null;unique;index:idx_unique_allocations_tx,unique"`
	TotalSize      int64            `gorm:"column:size;not null;default:0"`
	UsedSize       int64            `gorm:"column:used_size;not null;default:0"`
	OwnerID        string           `gorm:"column:owner_id;size:64;not null"`
	OwnerPublicKey string           `gorm:"column:owner_public_key;size:512;not null"`
	RepairerID     string           `gorm:"column:repairer_id;size:64;not null"`
	PayerID        string           `gorm:"column:payer_id;size:64;not null"`
	Expiration     common.Timestamp `gorm:"column:expiration_date;not null"`
	// AllocationRoot allcation_root of last write_marker
	AllocationRoot   string        `gorm:"column:allocation_root;size:64;not null;default:''"`
	BlobberSize      int64         `gorm:"column:blobber_size;not null;default:0"`
	BlobberSizeUsed  int64         `gorm:"column:blobber_size_used;not null;default:0"`
	LatestRedeemedWM string        `gorm:"column:latest_redeemed_write_marker;size:64"`
	IsRedeemRequired bool          `gorm:"column:is_redeem_required"`
	TimeUnit         time.Duration `gorm:"column:time_unit;not null;default:172800000000000"`
	IsImmutable      bool          `gorm:"is_immutable;not null"`
	// Ending and cleaning
	CleanedUp bool `gorm:"column:cleaned_up;not null;default:false"`
	Finalized bool `gorm:"column:finalized;not null;default:false"`
	// Has many terms
	// If Preload("Terms") is required replace tag `gorm:"-"` with `gorm:"foreignKey:AllocationID"`
	Terms []*Terms `gorm:"-"`
}

func (Allocation) TableName() string {
	return TableNameAllocation
}

// RestDurationInTimeUnits returns number (float point) of time units until
// allocation ends.
func (a *Allocation) RestDurationInTimeUnits(wmt common.Timestamp) (rdtu float64) {
	var (
		wmtt = time.Unix(int64(wmt), 0)
		expt = time.Unix(int64(a.Expiration), 0)
	)

	return float64(expt.Sub(wmtt)) / float64(a.TimeUnit)
}

func sizeInGB(size int64) float64 {
	return float64(size) / GB
}

// GetRequiredReadBalance Get tokens required to read the given size
func (a *Allocation) GetRequiredReadBalance(blobberID string, readSize int64) (value float64) {
	for _, d := range a.Terms {
		if d.BlobberID == blobberID {
			value = sizeInGB(readSize) * float64(d.ReadPrice)
			break
		}
	}
	return
}

// GetRequiredWriteBalance Get tokens required to write the give size
func (a *Allocation) GetRequiredWriteBalance(blobberID string, writeSize int64, wmt common.Timestamp) (value int64) {
	for _, d := range a.Terms {
		if d.BlobberID == blobberID {
			value = int64(sizeInGB(writeSize)*float64(d.WritePrice)) * int64(a.RestDurationInTimeUnits(wmt))
			break
		}
	}
	return
}

type Pending struct {
	// ID of format client_id:allocation_id
	ID           string `gorm:"column:id;primaryKey"`
	PendingWrite int64  `gorm:"column:pending_write;not null;default:0;"`
	PendingRead  int64  `gorm:"column:pending_read;pending_read;not null;default:0"` // size

}

func (*Pending) TableName() string {
	return "pendings"
}

// GetPendingWrite Get write size that is not yet redeemed
func GetPendingWrite(db *gorm.DB, clientID, allocationID string) (pendingWriteSize int64, err error) {
	err = db.Model(&Pending{}).Select("pending_write").Where(
		"id=?", fmt.Sprintf("%v:%v", clientID, allocationID),
	).Scan(&pendingWriteSize).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}
	return
}

// GetPendingRead Get read size that is not yet redeemed
func GetPendingRead(db *gorm.DB, clientID, allocationID string) (pendingReadSize int64, err error) {
	err = db.Model(&Pending{}).Select("pending_read").Where(
		"id=?", fmt.Sprintf("%v:%v", clientID, allocationID),
	).Scan(&pendingReadSize).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}
	return
}

func AddToPending(db *gorm.DB, clientID, allocationID string, pendingWrite, pendingRead int64) (err error) {
	key := clientID + ":" + allocationID
	// Lock is required because two process can simultaneously call this function and read pending data
	// thus giving same value leading to inconsistent data
	lock, _ := pendingMapLock.GetLock(key)
	lock.Lock()
	defer lock.Unlock()

	pending := new(Pending)
	err = db.Model(&Pending{}).Where("id=?", key).First(pending).Error
	switch {
	case err == nil:
		pending.PendingWrite += pendingWrite
		pending.PendingRead += pendingRead
		db.Save(pending)
	case errors.Is(err, gorm.ErrRecordNotFound):
		pending.ID = key
		pending.PendingWrite = pendingWrite
		pending.PendingRead = pendingRead
		db.Create(pending)
	default:
		return err
	}
	return nil
}

func GetWritePoolsBalance(db *gorm.DB, clientID, allocationID string, until common.Timestamp) (balance int64, err error) {
	err = db.Model(&WritePool{}).Select("sum (balance) as tot_balance").Where(
		"allocation_id = ? AND "+
			"client_id = ? AND "+
			"expire_at > ?", allocationID, clientID, until,
	).Scan(&balance).Error
	return
}

func (p *Pending) Save(tx *gorm.DB) error {
	return tx.Save(p).Error
}

const (
	TableNameTerms = "terms"
)

// Terms for allocation by its Tx.
type Terms struct {
	ID           int64      `gorm:"column:id;primaryKey"`
	BlobberID    string     `gorm:"blobber_id;size:64;not null"`
	AllocationID string     `gorm:"allocation_id;size:64;not null"`
	Allocation   Allocation `gorm:"foreignKey:AllocationID"` // references allocations(id)

	ReadPrice  int64 `gorm:"read_price;not null"`
	WritePrice int64 `gorm:"write_price;not null"`
}

func (Terms) TableName() string {
	return TableNameTerms
}

type ReadPool struct {
	PoolID       string `gorm:"column:pool_id;size:64;primaryKey"`
	ClientID     string `gorm:"column:client_id;size:64;not null;index:idx_read_pools_cab,priority:1"`
	AllocationID string `gorm:"column:allocation_id;size:64;not null;index:idx_read_pools_cab,priority:2"`
	// Cached balance in read pool. Might need update when balance - pending is less than 0
	Balance  int64            `gorm:"column:balance;not null"`
	ExpireAt common.Timestamp `gorm:"column:expire_at;not null"`
}

func (ReadPool) TableName() string {
	return "read_pools"
}

type WritePool struct {
	PoolID       string           `gorm:"column:pool_id;size:64;primaryKey"`
	ClientID     string           `gorm:"column:client_id;size:64;not null;index:idx_write_pools_cab,priority:1"`
	AllocationID string           `gorm:"column:allocation_id;size:64;not null;index:idx_write_pools_cab,priority:2"`
	Balance      int64            `gorm:"column:balance;not null"`
	ExpireAt     common.Timestamp `gorm:"column:expire_at;not null"`
}

func (WritePool) TableName() string {
	return "write_pools"
}

func GetReadPools(db *gorm.DB, allocationID, clientID string, until common.Timestamp) (rps []*ReadPool, err error) {
	err = db.Model(&ReadPool{}).Select("pool_id", "balance", "expire_at").
		Where(
			"allocation_id = ? AND "+
				"client_id = ? AND "+
				"expire_at > ?", allocationID, clientID, until).Find(&rps).Error

	if err != nil {
		return nil, err
	}
	return
}

func GetReadPoolsBalance(db *gorm.DB, allocationID, clientID string, until common.Timestamp) (balance int64, err error) {
	var b *int64 // pointer to int64 for possible total sum as null
	err = db.Model(&ReadPool{}).Select("sum(balance) as tot_balance").Where(
		"client_id = ? AND "+
			"allocation_id = ? AND "+
			"expire_at > ?", clientID, allocationID, until).Scan(&b).Error

	if err != nil {
		return 0, err
	}
	if b == nil {
		return 0, nil
	}
	return *b, nil
}

func SetReadPools(db *gorm.DB, clientID, allocationID string, rps []*ReadPool) (err error) {
	// cleanup and batch insert (remove old pools, add / update new)
	const query = `client_id = ? AND
			allocation_id = ?`

	var stub []*ReadPool
	err = db.Model(&ReadPool{}).
		Where(query, clientID, allocationID).
		Delete(&stub).Error
	if err != nil {
		return
	}

	if len(rps) == 0 {
		return
	}

	err = db.Model(&ReadPool{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pool_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"balance"}),
	}).Create(rps).Error
	return
}

func SetWritePools(db *gorm.DB, clientID, allocationID string, wps []*WritePool) (err error) {
	const query = `client_id = ? AND
				allocation_id = ?`

	var stub []*WritePool
	err = db.Model(&WritePool{}).
		Where(query, clientID, allocationID).
		Delete(&stub).Error
	if err != nil {
		return
	}

	if len(wps) == 0 {
		return
	}

	err = db.Model(&WritePool{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "pool_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"balance"}),
	}).Create(wps).Error
	return
}

// ReadPoolRedeem represents read marker redeeming transaction response with
// reductions of read pools. It allows to not refresh read pools from 0chain
// REST API every time and use cache in DB. The transaction returns list of
// the ReadPoolRedeems as JSON (e.g. [{...}, ..]).
type ReadPoolRedeem struct {
	PoolID  string `json:"pool_id"` // read pool ID
	Balance int64  `json:"balance"` // balance reduction
}

// SubReadRedeemed subtracts tokens redeemed from read pools.
func SubReadRedeemed(rps []*ReadPool, redeems []ReadPoolRedeem) {
	var rm = make(map[string]int64)

	for _, rd := range redeems {
		rm[rd.PoolID] += rd.Balance
	}

	for _, rp := range rps {
		if sub, ok := rm[rp.PoolID]; ok {
			rp.Balance -= sub
		}
	}
}
