package stats

import (
	"time"

	"0chain.net/blobbercore/filestore"
)

// Timestamp that implements standard fmt.Stringer interface.
type Timestamp int64

// String implements standard fmt.Stringer interface.
func (t Timestamp) String() string {
	return time.Unix(int64(t), 0).String()
}

type AllocationStats struct {
	AllocationID   string `json:"allocation_id"`
	TempFolderSize int64  `json:"-"`
	Stats
	Expiration Timestamp `json:"expiration_date" gorm:"column:expiration_date"`

	ReadMarkers  *ReadMarkersStat  `json:"read_markers"`
	WriteMarkers *WriteMarkersStat `json:"write_markers"`
}

func (fs *AllocationStats) loadAllocationDiskUsageStats() error {
	du, err := filestore.GetFileStore().GetlDiskSizeUsed(fs.AllocationID)
	if err != nil {
		du = -1
	}
	fs.DiskSizeUsed = du
	tfs, err := filestore.GetFileStore().GetTempPathSize(fs.AllocationID)
	if err != nil {
		tfs = -1
	}
	fs.TempFolderSize = tfs
	return err
}
