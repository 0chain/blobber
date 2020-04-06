package allocation

import (
	"0chain.net/core/common"
)

const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB
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
	// WritePrice is average write price of all blobbers of the allocation.
	WritePrice float64 `gorm:"column:write_price"`
	// ReadPrice is average read price of all blobbers of the allocation.
	ReadPrice float64 `gorm:"column:read_price"`
	// NumBlobbers is number of blobbers of the allocation.
	NumBlobbers int64 `gorm:"column:num_blobbers"`
}

func (Allocation) TableName() string {
	return "allocations"
}

func sizeInGB(size int64) float64 {
	return float64(size) / GB
}

// WriteValue is number of tokens required to write given size.
func (a *Allocation) WriteValue(size int64) int64 {
	return int64(sizeInGB(size) * a.WritePrice * float64(a.NumBlobbers))
}

// ReadValue of tokens locked in a read pool.
func (a *Allocation) ReadValue(blocks int64) int64 {
	return int64(sizeInGB(blocks*64*KB) * a.ReadPrice * float64(a.NumBlobbers))
}
