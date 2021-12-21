package disk_balancer

import (
	"fmt"

	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type (
	// DiskSelector represented by the disk balancer.
	DiskSelector interface {
		// GetNextVolumePath selects a root directory for storing data.
		GetNextVolumePath(fileSize int64) (string, error)
	}
)

var diskSelector DiskSelector

// NewDiskSelector represented instance of the DiskSelector interface.
func NewDiskSelector() {
	dTier := &diskTier{}
	if err := dTier.init(); err != nil {
		Logger.Error(fmt.Sprintf("NewDiskSelector() %v", err))
		return
	}
	diskSelector = dTier
	return
}

func GetDiskSelector() DiskSelector {
	return diskSelector
}
