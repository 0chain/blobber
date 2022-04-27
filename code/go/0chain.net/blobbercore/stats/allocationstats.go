package stats

import (
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

// Timestamp that implements standard fmt.Stringer interface.
type Timestamp int64

// String implements standard fmt.Stringer interface.
func (t Timestamp) String() string {
	return time.Unix(int64(t), 0).String()
}

type AllocationStats struct {
	AllocationID   string `json:"allocation_id"`
	TempFolderSize uint64 `json:"-"`
	Stats
	Expiration Timestamp `json:"expiration_date" gorm:"column:expiration_date"`

	ReadMarkers  *ReadMarkersStat  `json:"read_markers"`
	WriteMarkers *WriteMarkersStat `json:"write_markers"`
}

func (aStat *AllocationStats) loadAllocationDiskUsageStats() error {
	aStat.DiskSizeUsed = filestore.GetFileStore().GetPermFilesSizeByAllocation(aStat.AllocationID)
	aStat.TempFolderSize = filestore.GetFileStore().GetTempFilesSizeByAllocation(aStat.AllocationID)
	return nil
}
