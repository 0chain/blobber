package disk_balancer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/shirou/gopsutil/disk"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
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
		physicalSize   int64
	}

	partition struct {
		availableSize int64
		path          string
		physicalSize  int64
	}
)

const (
	// MinSizeFirst the strategy of choosing the directory with the least occupied space.
	MinSizeFirst = "min_size_first"
	// TempAllocationFile represent a name of the file controlling the transfer Allocation.
	TempAllocationFile = "relocatable.json"

	OSPathSeparator = string(os.PathSeparator)
	UserFiles       = "files"
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
	partitionStats, _ := disk.Partitions(false)
	reg := regexp.MustCompile(d.mountPoint)
	log.Println("REG = ", reg)
	for _, partitionStat := range partitionStats {
		log.Println(partitionStat)
		log.Println(partitionStat.Mountpoint)
		if reg.MatchString(partitionStat.Mountpoint) {
			if !d.canUsed(partitionStat.Mountpoint) {
				continue
			}
			dirs := filepath.Join(partitionStat.Mountpoint, UserFiles)
			if err := d.createDirs(dirs); err != nil {
				continue
			}
			dParts = append(dParts, dirs)
		}
	}
	var physicalSize int64
	for _, pathPartition := range dParts {
		vol, err := d.updatePartitionInfo(pathPartition)
		if err != nil {
			continue
		}
		partitions[pathPartition] = vol
		physicalSize += vol.physicalSize
		d.physicalSize = physicalSize
	}
	if len(partitions) == 0 {
		Logger.Error("checkDisks(): no disk for storage users files")
		d.partitions = partitions
		return
	}

	d.partitions = partitions
}

// checkDisksWorker checks disks on a schedule.
func (d *diskTier) checkDisksWorker(ctx context.Context) {
	fmt.Println("select disk worker is running")
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

// checkUndeletedFiles is run when disk balancer starts. Checks for unreleased copies of allocation and deletes them.
func (d *diskTier) checkUndeletedFiles() {
	for part := range d.partitions {
		files, err := ioutil.ReadDir(part)
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
	fmt.Fprintf(&dir, "%s%s", root, OSPathSeparator)
	for i := 0; i < 3; i++ {
		fmt.Fprintf(&dir, "%s%s", OSPathSeparator, transID[3*i:3*i+3])
	}
	fmt.Fprintf(&dir, "%s%s", OSPathSeparator, transID[9:])

	return filepath.Clean(dir.String())
}

// GetNextDiskPath implemented DiskSelector interface.
func (d *diskTier) GetNextDiskPath() (string, error) {
	return d.selectNextDisk(d.partitions)
}

// GetListDisks implemented DiskSelector interface.
func (d *diskTier) GetListDisks() []string {
	var s []string
	for _, p := range d.partitions {
		s = append(s, p.path)
	}

	return s
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

		return "", errors.New("not enough disk spase")
	}

	return path, nil
}

func (d *diskTier) GetCapacity() int64 {
	return d.physicalSize
}

// init initializes the listed directories and registers the write strategy.
func (d *diskTier) init(ctx context.Context) error {
	d.minDiskSize = config.Configuration.MinDiskSize
	d.mountPoint = config.Configuration.MountPoint
	d.checkTimeOut = config.Configuration.CheckDisksTimeout
	d.selectNextDisk = d.selectStrategy(config.Configuration.Strategy)

	d.checkDisks()
	if len(d.partitions) == 0 {
		return errors.New("init() no disk for storage users files")
	}
	go d.checkDisksWorker(ctx)

	return nil
}

// IsMoves implemented DiskSelector interface.
func (d *diskTier) IsMoves(allocationRoot, allocationID string, needPath bool) (bool, string) {
	path := d.generateAllocationPath(allocationRoot, allocationID)
	fPath := filepath.Join(path, TempAllocationFile)

	if _, err := os.Stat(fPath); os.IsNotExist(err) {
		return false, allocationRoot
	}

	if needPath {
		a := readFile(fPath)
		return true, a.NewRoot
	}

	return true, ""
}

// MoveAllocation implemented DiskSelector interface.
func (d *diskTier) MoveAllocation(srcPath, destPath, transID string) string {
	return d.moveAllocation(srcPath, destPath, transID, common.GetRootContext())
}

// moveAllocation moved allocation.
func (d *diskTier) moveAllocation(srcPath, destPath, transID string, ctx context.Context) string {
	oldAllocationPath := d.generateAllocationPath(srcPath, transID)
	aInfo := newAllocationInfo(
		oldAllocationPath,
		destPath,
	)
	// d.generateAllocationPath(destPath, transID),
	if err := aInfo.prepareAllocation(); err != nil {
		Logger.Error("prepareAllocation() failed", zap.Error(err))
	}

	if err := aInfo.move(ctx); err != nil {
		Logger.Error("move() failed", zap.Error(err))
	}

	deleteAllocation(oldAllocationPath)

	return aInfo.NewRoot
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
		Logger.Error(fmt.Sprintf("updatePartitionInfo() filed %v", err))
		return nil, err
	}

	return vol, nil
}

// getAvailableSize gets information about the state of a directory.
func (p *partition) getAvailableSize() error {
	var volStat unix.Statfs_t
	err := unix.Statfs(p.path, &volStat)
	if err != nil {
		Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return err
	}
	p.physicalSize = int64(volStat.Bavail * uint64(volStat.Bsize))
	p.availableSize = int64(volStat.Bfree * uint64(volStat.Bsize))

	return nil
}

func (p *partition) GetPartitionPath() string {

	return ""
}
