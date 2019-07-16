package stats

import (
	"0chain.net/blobbercore/filestore"
)

type AllocationStats struct {
	AllocationID   string `json:"allocation_id"`
	TempFolderSize int64  `json:"-"`
	Stats
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
