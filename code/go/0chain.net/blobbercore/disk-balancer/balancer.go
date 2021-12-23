package disk_balancer

import (
	"context"
	"fmt"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type (
	// DiskSelector represented by the disk balancer.
	DiskSelector interface {
		// GetAvailableDisk return available disk for allocation.
		GetAvailableDisk(path string, size int64) (diskPath string, err error)
		// GetNextDiskPath selects a disk for storing data.
		GetNextDiskPath() (string, error)
		// MoveAllocation moved allocation to another disk.
		MoveAllocation(srcPath, destPath, transID string) error
	}
)

var diskSelector DiskSelector

// StartDiskSelectorWorker represented instance of the DiskSelector interface.
func StartDiskSelectorWorker(ctx context.Context) {
	dTier := &diskTier{}
	if err := dTier.init(ctx); err != nil {
		Logger.Error(fmt.Sprintf("StartDiskSelectorWorker() %v", err))
		return
	}
	diskSelector = dTier
	return
}

func GetDiskSelector() DiskSelector {
	return diskSelector
}
