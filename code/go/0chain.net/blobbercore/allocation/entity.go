package allocation

import (
	"errors"
	"time"

	"0chain.net/core/common"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB

	CHUNK_SIZE = 64 * KB
)

type Allocation struct {
	ID               string           `gorm:"column:id;primary_key"`
	Tx               string           `gorm:"column:tx"`
	TotalSize        int64            `gorm:"column:size"`
	UsedSize         int64            `gorm:"column:used_size"`
	OwnerID          string           `gorm:"column:owner_id"`
	OwnerPublicKey   string           `gorm:"column:owner_public_key"`
	PayerID          string           `gorm:"column:payer_id"` // optional / client paying for all r/w ops
	Expiration       common.Timestamp `gorm:"column:expiration_date"`
	AllocationRoot   string           `gorm:"column:allocation_root"`
	BlobberSize      int64            `gorm:"column:blobber_size"`
	BlobberSizeUsed  int64            `gorm:"column:blobber_size_used"`
	LatestRedeemedWM string           `gorm:"column:latest_redeemed_write_marker"`
	IsRedeemRequired bool             `gorm:"column:is_redeem_required"`
	TimeUnit         time.Duration    `gorm:"column:time_unit"`
	// ending and cleaning
	CleanedUp        bool `gorm:"column:cleaned_up"`
	Finalized        bool `gorm:"column:finalized"`
	// Has many terms.
	Terms            []*Terms `gorm:"-"`
}

func (Allocation) TableName() string {
	return "allocations"
}

// RestDurationInTimeUnits returns number (float point) of time units until
// allocation ends.
func (a *Allocation) RestDurationInTimeUnits(wmt common.Timestamp) (
	rdtu float64) {

	var (
		wmtt = time.Unix(int64(wmt), 0)
		expt = time.Unix(int64(a.Expiration), 0)
	)

	return float64(expt.Sub(wmtt)) / float64(a.TimeUnit)
}

func sizeInGB(size int64) float64 {
	return float64(size) / GB
}

// WantReader implements WantRead that returns cost of given numBlocks
// for given blobber.
type WantReader interface {
	WantRead(blobberID string, numBlocks int64) (value int64) // the want read
}

// WantRead returns amount of tokens (by current terms of the allocations that
// should be loaded) by given number of blocks for given blobber. E.g. want is
// tokens wanted.
func (a *Allocation) WantRead(blobberID string, numBlocks int64) (value int64) {
	for _, d := range a.Terms {
		if d.BlobberID == blobberID {
			value = int64(sizeInGB(numBlocks*CHUNK_SIZE) * float64(d.ReadPrice))
			break
		}
	}
	return
}

// WantWriter implements WantWrite that returns cost of given size in bytes
// for given blobber.
type WantWriter interface {
	WantWrite(blobberID string, size int64, wmt common.Timestamp) (value int64)
}

// WantWrite returns amount of tokens (by current terms of the allocations that
// should be loaded) by given size for given blobber. E.g. want is tokens
// wanted.
func (a *Allocation) WantWrite(blobberID string, size int64,
	wmt common.Timestamp) (value int64) {

	if size < 0 {
		return // deleting, ignore
	}

	for _, d := range a.Terms {
		if d.BlobberID == blobberID {
			value = int64(sizeInGB(size) * float64(d.WritePrice) *
				a.RestDurationInTimeUnits(wmt))
			break
		}
	}

	return
}

// ReadPools from DB cache.
func ReadPools(tx *gorm.DB, clientID, allocID, blobberID string,
	until common.Timestamp) (rps []*ReadPool, err error) {

	const query = `client_id = ? AND
        allocation_id = ? AND
        blobber_id = ? AND
        expire_at > ?`

	err = tx.Model(&ReadPool{}).
		Where(query, clientID, allocID, blobberID, until).
		Find(&rps).Error
	return
}

// HaveRead is sum of read pools (the list should be filtered by query
// excluding pools expired and pools going to expired soon) minus pending reads.
func (a *Allocation) HaveRead(rps []*ReadPool, blobberID string,
	pendNumBlocks int64) (have int64) {

	for _, rp := range rps {
		have += rp.Balance
	}
	return have - a.WantRead(blobberID, pendNumBlocks)
}

type Pending struct {
	ID int64 `gorm:"column:id;primary_key"`

	ClientID     string `gorm:"column:client_id"`
	AllocationID string `gorm:"column:allocation_id"`
	BlobberID    string `gorm:"column:blobber_id"`

	PendingWrite int64 `gorm:"column:pending_write"` // size
}

func (*Pending) TableName() string {
	return "pendings"
}

func GetPending(tx *gorm.DB, clientID, allocationID, blobberID string) (
	p *Pending, err error) {

	const query = `client_id = ? AND
        allocation_id = ? AND
        blobber_id = ?`

	p = new(Pending)
	err = tx.Model(&Pending{}).
		Where(query, clientID, allocationID, blobberID).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p.ClientID = clientID
		p.AllocationID = allocationID
		p.BlobberID = blobberID
		err = tx.Create(p).Error
	}
	return
}

func (p *Pending) AddPendingWrite(size int64) {
	p.PendingWrite += size
}

func (p *Pending) SubPendingWrite(size int64) {
	if p.PendingWrite -= size; p.PendingWrite < 0 {
		p.PendingWrite = 0
	}
}

func (p *Pending) WritePools(tx *gorm.DB, blobberID string,
	until common.Timestamp) (wps []*WritePool, err error) {

	const query = `client_id = ? AND
        allocation_id = ? AND
        blobber_id = ? AND
        expire_at > ?`

	err = tx.Model(&WritePool{}).
		Where(query, p.ClientID, p.AllocationID, blobberID, until).
		Find(&wps).Error
	return
}

func (p *Pending) HaveWrite(wps []*WritePool, ww WantWriter,
	wmt common.Timestamp) (have int64) {

	for _, wp := range wps {
		have += wp.Balance
	}
	return have - ww.WantWrite(p.BlobberID, p.PendingWrite, wmt)
}

func (p *Pending) Save(tx *gorm.DB) error {
	if p.ID == 0 {
		return tx.Create(p).Error
	}
	return tx.Save(p).Error
}

// Terms for allocation by its Tx.
type Terms struct {
	ID           int64  `gorm:"column:id;primary_key"`
	BlobberID    string `gorm:"blobber_id"`
	AllocationID string `gorm:"allocation_id"`

	ReadPrice  int64 `gorm:"read_price"`
	WritePrice int64 `gorm:"write_price"`
}

func (*Terms) TableName() string {
	return "terms"
}

type ReadPool struct {
	PoolID string `gorm:"column:pool_id;primary_key"`

	ClientID     string `gorm:"column:client_id"`
	BlobberID    string `gorm:"column:blobber_id"`
	AllocationID string `gorm:"column:allocation_id"`

	Balance  int64            `gorm:"column:balance"`
	ExpireAt common.Timestamp `gorm:"column:expire_at"`
}

func (*ReadPool) TableName() string {
	return "read_pools"
}

type WritePool struct {
	PoolID string `gorm:"column:pool_id;primary_key"`

	ClientID     string `gorm:"column:client_id"`
	BlobberID    string `gorm:"column:blobber_id"`
	AllocationID string `gorm:"column:allocation_id"`

	Balance  int64            `gorm:"column:balance"`
	ExpireAt common.Timestamp `gorm:"column:expire_at"`
}

func (*WritePool) TableName() string {
	return "write_pools"
}

func SetReadPools(db *gorm.DB, clientID, allocationID, blobberID string,
	rps []*ReadPool) (err error) {

	// cleanup and batch insert (remove old pools, add / update new)

	const query = `client_id = ? AND
        allocation_id = ? AND
        blobber_id = ?`

	var stub []*ReadPool
	err = db.Model(&ReadPool{}).
		Where(query, clientID, allocationID, blobberID).
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

func SetWritePools(db *gorm.DB, clientID, allocationID, blobberID string,
	wps []*WritePool) (err error) {

	const query = `client_id = ? AND
        allocation_id = ? AND
        blobber_id = ?`

	var stub []*WritePool
	err = db.Model(&WritePool{}).
		Where(query, clientID, allocationID, blobberID).
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
