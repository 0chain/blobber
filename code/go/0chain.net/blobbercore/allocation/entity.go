package allocation

import (
	"errors"
	"fmt"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"

	"gorm.io/gorm"
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
var pendingMapLock = common.GetNewLocker()

const (
	TableNameAllocation = "allocations"
	CanUploadMask       = uint16(1)  // 0000 0001
	CanDeleteMask       = uint16(2)  // 0000 0010
	CanUpdateMask       = uint16(4)  // 0000 0100
	CanMoveMask         = uint16(8)  // 0000 1000
	CanCopyMask         = uint16(16) // 0001 0000
	CanRenameMask       = uint16(32) // 0010 0000
)

type Allocation struct {
	ID             string           `gorm:"column:id;size:64;primaryKey"`
	Tx             string           `gorm:"column:tx;size:64;not null;unique;index:idx_unique_allocations_tx,unique"`
	TotalSize      int64            `gorm:"column:size;not null;default:0"`
	UsedSize       int64            `gorm:"column:used_size;not null;default:0"`
	OwnerID        string           `gorm:"column:owner_id;size:64;not null"`
	OwnerPublicKey string           `gorm:"column:owner_public_key;size:512;not null"`
	RepairerID     string           `gorm:"column:repairer_id;size:64;not null"`
	Expiration     common.Timestamp `gorm:"column:expiration_date;not null"`
	// AllocationRoot allcation_root of last write_marker
	AllocationRoot   string        `gorm:"column:allocation_root;size:64;not null;default:''"`
	FileMetaRoot     string        `gorm:"column:file_meta_root;size:64;not null;default:''"`
	BlobberSize      int64         `gorm:"column:blobber_size;not null;default:0"`
	BlobberSizeUsed  int64         `gorm:"column:blobber_size_used;not null;default:0"`
	LatestRedeemedWM string        `gorm:"column:latest_redeemed_write_marker;size:64"`
	IsRedeemRequired bool          `gorm:"column:is_redeem_required"`
	TimeUnit         time.Duration `gorm:"column:time_unit;not null;default:172800000000000"`
	// Ending and cleaning
	CleanedUp bool `gorm:"column:cleaned_up;not null;default:false"`
	Finalized bool `gorm:"column:finalized;not null;default:false"`

	// FileOptions to define file restrictions on an allocation for third-parties
	// default 00000000 for all crud operations suggesting only owner has the below listed abilities.
	// enabling option/s allows any third party to perform certain ops
	// 00000001 - 1  - upload
	// 00000010 - 2  - delete
	// 00000100 - 4  - update
	// 00001000 - 8  - move
	// 00010000 - 16 - copy
	// 00100000 - 32 - rename
	FileOptions uint16 `json:"file_options" gorm:"column:file_options;not null;default:63"`

	// Has many terms
	// If Preload("Terms") is required replace tag `gorm:"-"` with `gorm:"foreignKey:AllocationID"`
	Terms []*Terms `gorm:"-"`
}

func (Allocation) TableName() string {
	return TableNameAllocation
}

func (a *Allocation) CanUpload() bool {
	return (a.FileOptions & CanUploadMask) > 0
}

func (a *Allocation) CanDelete() bool {
	return (a.FileOptions & CanDeleteMask) > 0

}

func (a *Allocation) CanUpdate() bool {
	return (a.FileOptions & CanUpdateMask) > 0
}

func (a *Allocation) CanMove() bool {
	return (a.FileOptions & CanMoveMask) > 0
}

func (a *Allocation) CanCopy() bool {
	return (a.FileOptions & CanCopyMask) > 0
}

func (a *Allocation) CanRename() bool {
	return (a.FileOptions & CanRenameMask) > 0
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
func (a *Allocation) GetRequiredReadBalance(blobberID string, numBlocks int64) (value float64) {
	for _, d := range a.Terms {
		if d.BlobberID == blobberID {
			value = sizeInGB(numBlocks*CHUNK_SIZE) * float64(d.ReadPrice)
			break
		}
	}
	return
}

// GetRequiredWriteBalance Get tokens required to write the give size
func (a *Allocation) GetRequiredWriteBalance(blobberID string, writeSize int64, wmt common.Timestamp) (value uint64) {
	for _, d := range a.Terms {
		if d.BlobberID == blobberID {

			value = uint64(sizeInGB(writeSize)*float64(d.WritePrice)) * uint64(a.RestDurationInTimeUnits(wmt))
			logging.Logger.Info("jayash GetRequiredWriteBalance", zap.Any("d.WritePrice", d.WritePrice), zap.Any("writeSize", writeSize), zap.Any("a.RestDurationInTimeUnits(wmt)", a.RestDurationInTimeUnits(wmt)), zap.Any("value", value))
			break
		}
	}
	return
}

type Pending struct {
	// ID of format client_id:allocation_id
	ID           string `gorm:"column:id;primaryKey"`
	PendingWrite int64  `gorm:"column:pending_write;not null;default:0;"`
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

func AddToPending(db *gorm.DB, clientID, allocationID string, pendingWrite int64) (err error) {
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
		db.Save(pending)
	case errors.Is(err, gorm.ErrRecordNotFound):
		pending.ID = key
		pending.PendingWrite = pendingWrite
		db.Create(pending)
	default:
		return err
	}
	return nil
}

func GetWritePoolsBalance(db *gorm.DB, allocationID string) (balance uint64, err error) {
	err = db.Model(&WritePool{}).Select("COALESCE(SUM(balance),0) as tot_balance").Where(
		"allocation_id = ?", allocationID,
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

	ReadPrice  uint64 `gorm:"read_price;not null"`
	WritePrice uint64 `gorm:"write_price;not null"`
}

func (Terms) TableName() string {
	return TableNameTerms
}

// ReadPool represents new trimmed down readPool consisting of two balances,
// one for the allocations that the client (client_id) owns
// and the other for the allocations that the client (client_id) doesn't own
type ReadPool struct {
	ClientID string `gorm:"column:client_id;size:64;primaryKey" json:"client_id"`
	Balance  int64  `gorm:"column:balance;not null" json:"balance"`
}

func (ReadPool) TableName() string {
	return "read_pools"
}

type WritePool struct {
	AllocationID string `gorm:"column:allocation_id;size:64;not null;index:idx_write_pools_cab,priority:1"`
	Balance      uint64 `gorm:"column:balance;not null"`
}

func (WritePool) TableName() string {
	return "write_pools"
}

func GetReadPool(db *gorm.DB, clientID string) (*ReadPool, error) {
	var rp ReadPool
	return &rp, db.Model(&ReadPool{}).Where("client_id = ?", clientID).Scan(&rp).Error
}

func GetReadPoolsBalance(db *gorm.DB, clientID string) (int64, error) {
	rp, err := GetReadPool(db, clientID)
	if err != nil {
		return 0, err
	}

	return rp.Balance, nil
}

func SetReadPool(db *gorm.DB, rp *ReadPool) error {
	var erp ReadPool //find existing read pool
	err := db.Model(&ReadPool{}).Where("client_id = ?", rp.ClientID).FirstOrCreate(&erp, rp).Error
	if err != nil {
		return err
	}

	if erp.Balance == rp.Balance {
		return nil
	}
	// update existing
	return UpdateReadPool(db, rp)
}

func UpdateReadPool(db *gorm.DB, rp *ReadPool) error {
	return db.Model(&ReadPool{}).Where("client_id = ?", rp.ClientID).Updates(map[string]interface{}{
		"balance": rp.Balance,
	}).Error
}

func SetWritePool(db *gorm.DB, allocationID string, wp *WritePool) (err error) {
	err = db.Delete(&WritePool{}, "allocation_id = ?", allocationID).Error
	if err != nil {
		return
	}

	if wp == nil {
		return
	}

	err = db.Create(wp).Error
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
