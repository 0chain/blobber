package allocation

import (
	"0chain.net/core/common"
)

type Allocation struct {
	ID               string           `gorm:"column:id;primary_key"`
	TotalSize        int64            `gorm:"column:size"`
	UsedSize         int64            `gorm:"column:used_size"`
	OwnerID          string           `gorm:"column:owner_id"`
	OwnerPublicKey   string           `gorm:"column:owner_public_key"`
	Expiration       common.Timestamp `gorm:"column:expiration_date"`
	AllocationRoot   string           `gorm:"column:allocation_root"`
	BlobberSize      int64            `gorm:"column:blobber_size"`
	BlobberSizeUsed  int64            `gorm:"column:blobber_size_used"`
	LatestRedeemedWM string           `gorm:"column:latest_redeemed_write_marker"`
	IsRedeemRequired bool             `gorm:"column:is_redeem_required"`
}

func (Allocation) TableName() string {
	return "allocations"
}
