package models

import (
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/core/common"
)

const (
	TableNameAllocation = "allocations"
)

// Allocation DTO: allocation
type Allocation struct {
	ID             string           `gorm:"column:id;primary_key"`
	Tx             string           `gorm:"column:tx"`
	TotalSize      int64            `gorm:"column:size"`
	UsedSize       int64            `gorm:"column:used_size"`
	OwnerID        string           `gorm:"column:owner_id"`
	OwnerPublicKey string           `gorm:"column:owner_public_key"`
	RepairerID     string           `gorm:"column:repairer_id"` // experimental / blobber node id
	PayerID        string           `gorm:"column:payer_id"`    // optional / client paying for all r/w ops
	Expiration     common.Timestamp `gorm:"column:expiration_date"`
	// AllocationRoot allcation_root of last write_marker
	AllocationRoot   string        `gorm:"column:allocation_root"`
	BlobberSize      int64         `gorm:"column:blobber_size"`
	BlobberSizeUsed  int64         `gorm:"column:blobber_size_used"`
	LatestRedeemedWM string        `gorm:"column:latest_redeemed_write_marker"`
	IsRedeemRequired bool          `gorm:"column:is_redeem_required"`
	TimeUnit         time.Duration `gorm:"column:time_unit"`
	IsImmutable      bool          `gorm:"is_immutable"`
	// Ending and cleaning
	CleanedUp bool `gorm:"column:cleaned_up"`
	Finalized bool `gorm:"column:finalized"`
}

func (Allocation) TableName() string {
	return TableNameAllocation
}
