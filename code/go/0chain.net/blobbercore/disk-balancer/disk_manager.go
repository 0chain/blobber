package disk_balancer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/disk"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/allocation"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type (
	diskTier struct {
		checkTimeOut   time.Duration
		minDiskSize    uint64
		mountPoint     string
		selectNextDisk func(partitions map[string]*partition) (string, error)
		partitions     map[string]*partition
	}

	partition struct {
		availableSize int64
		path          string
	}
)

const (
	// MinSizeFirst the strategy of choosing the directory with the least occupied space.
	MinSizeFirst = "min_size_first"
	// TempAllocationFile represent a name of the file controlling the transfer Allocation.
	TempAllocationFile = "relocatable.json"
)

// canUsed check min disk size
func (d *diskTier) canUsed(path string) bool {
	var volStat unix.Statfs_t
	err := unix.Statfs(path, &volStat)
	if err != nil {
		Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return false
	}

	if d.minDiskSize > volStat.Blocks*uint64(volStat.Bsize) {
		return false
	}

	return true
}

// checkDisks checks for sufficient space and write-ability in the listed directories.
func (d *diskTier) checkDisks() {
	var dParts []string
	partitions := make(map[string]*partition)
	disks, _ := disk.Partitions(false)
	reg := regexp.MustCompile(d.mountPoint)
	for _, disk := range disks {
		if reg.MatchString(disk.Mountpoint) {
			if !d.canUsed(disk.Mountpoint) {
				continue
			}
			dirs := filepath.Join(disk.Mountpoint, filestore.UserFiles)
			if err := d.createDirs(dirs); err != nil {
				continue
			}
			dParts = append(dParts, disk.Mountpoint)
		}
	}
	for _, pathPartition := range dParts {
		vol, err := d.updatePartitionInfo(pathPartition)
		if err != nil {
			continue
		}
		partitions[pathPartition] = vol
	}
	if len(partitions) == 0 {
		Logger.Error("checkDisks(): no disk for storage users files")
		d.partitions = partitions
		return
	}

	d.partitions = partitions

	return
}

// checkDisksWorker checks disks on a schedule.
func (d *diskTier) checkDisksWorker(ctx context.Context) {
	timer := time.NewTicker(d.checkTimeOut)
	for {
		select {
		case <-timer.C:
			d.checkDisks()
		case <-ctx.Done():
			return
		}
	}
}

func (d *diskTier) createDirs(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			return err
		}
	}
	return nil
}

// generateAllocationPath generated path to allocation be transaction ID.
func (d *diskTier) generateAllocationPath(root, transID string) string {
	var dir bytes.Buffer
	fmt.Fprintf(&dir, "%s%s", root, filestore.OSPathSeperator)
	fmt.Fprintf(&dir, "%s%s", filestore.OSPathSeperator, transID[0:3])

	return dir.String()
}

// GetNextDiskPath implemented DiskSelector interface.
func (d *diskTier) GetNextDiskPath() (string, error) {
	return d.selectNextDisk(d.partitions)
}

// GetAvailableDisk implemented DiskSelector interface.
func (d *diskTier) GetAvailableDisk(path string, size int64) (string, error) {
	vol := d.partitions[path]
	if vol.availableSize < size {
		for k, v := range d.partitions {
			if v.availableSize < size {
				continue
			}
			return k, nil
		}

		// TODO what to do if there is a total disk space available
		return "", errors.New("not enough disk spase")
	}

	return path, nil
}

// init initializes the listed directories and registers the write strategy.
func (d *diskTier) init(ctx context.Context) error {
	d.minDiskSize = config.Configuration.MinDiskSize
	d.mountPoint = config.Configuration.MountPoint
	d.selectNextDisk = d.selectStrategy(config.Configuration.Strategy)
	d.checkDisks()
	if len(d.partitions) == 0 {
		return errors.New("init() no disk for storage users files")
	}
	d.checkDisksWorker(ctx)

	return nil
}

// IsMoves implemented DiskSelector interface.
func (d *diskTier) IsMoves(ctx context.Context, allocationID string, needPath bool) (bool, string) {
	alloc, _ := allocation.VerifyAllocationTransaction(ctx, allocationID, true)
	path := filepath.Join(alloc.AllocationRoot, allocationID[:3], TempAllocationFile)
	if _, err := os.Stat(path); os.IsExist(err) {
		if needPath {
			a := readFile(path)
			return true, a.NewRoot
		}

		return true, ""
	}

	return false, ""
}

// MoveAllocation implemented DiskSelector interface.
func (d *diskTier) MoveAllocation(allocation *allocation.Allocation, destPath, transID string) {
	go d.moveAllocation(allocation, destPath, transID, common.GetRootContext())
}

// moveAllocation moved allocation.
func (d *diskTier) moveAllocation(alloc *allocation.Allocation, destPath, transID string, ctx context.Context) {
	oldAllocationPath := d.generateAllocationPath(alloc.AllocationRoot, transID)
	aInfo := newAllocationInfo(
		oldAllocationPath,
		d.generateAllocationPath(destPath, transID),
	)

	if err := aInfo.PrepareAllocation(); err != nil {
		Logger.Error("PrepareAllocation() failed", zap.Error(err))
	}

	if err := aInfo.Move(ctx); err != nil {
		Logger.Error("Move() failed", zap.Error(err))
	}

	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)

	a := new(allocation.Allocation)
	err := db.Model(&allocation.Allocation{}).
		Where(&allocation.Allocation{Tx: transID}).
		First(a).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		Logger.Error("moveAllocation() bad_db_operation", zap.Error(err)) // unexpected DB error
	}

	a.AllocationRoot = destPath
	if err = db.Save(a).Error; err != nil {
		Logger.Error("Failed to update allocation root path", zap.Error(err))
		db.Rollback()
		ctx.Done()
	}

	db.Commit()
	ctx.Done()

	deleteAllocation(oldAllocationPath)
}

// selectStrategy registers a function for selecting directories for storage.
func (d *diskTier) selectStrategy(strategy string) func(partitions map[string]*partition) (string, error) {
	switch strategy {
	case MinSizeFirst:
		return func(partitions map[string]*partition) (string, error) {
			var minSize int64 = -1
			var partitionPath string
			for k, p := range partitions {
				if err := p.getAvailableSize(); err != nil {
					continue
				}
				if p.availableSize > minSize {
					minSize = p.availableSize
					partitionPath = k
				}
			}

			if partitionPath == "" {
				return "", errors.New("no disk for storage users files")
			}

			return partitionPath, nil
		}
	default:
		panic(errors.New("no disk for storage users files"))
	}
}

// updatePartitionInfo updated list volumes
func (d *diskTier) updatePartitionInfo(volumePath string) (*partition, error) {
	vol := &partition{path: volumePath}
	if err := vol.getAvailableSize(); err != nil {
		Logger.Error(fmt.Sprintf("checkDisks() filed %v", err))
		return nil, err
	}

	return vol, nil
}

func (d *diskTier) checkUndeletedFiles() {
	for disk, _ := range d.partitions {
		files, err := ioutil.ReadDir(disk)
		if err != nil {
			Logger.Error("Failed checkUndeletedFiles", zap.Error(err))
		}
		for _, alloc := range files {
			if !alloc.IsDir() {
				continue
			}
			fPath := filepath.Join(alloc.Name(), TempAllocationFile)
			if _, err = os.Stat(fPath); os.IsExist(err) {
				a := readFile(fPath)
				if a.ForDelete {
					deleteAllocation(alloc.Name())
				}
			}
		}
	}
}

// getAvailableSize gets information about the state of a directory.
func (p *partition) getAvailableSize() error {
	var volStat unix.Statfs_t
	err := unix.Statfs(p.path, &volStat)
	if err != nil {
		Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return err
	}

	p.availableSize = int64(volStat.Bfree * uint64(volStat.Bsize))

	return nil
}
