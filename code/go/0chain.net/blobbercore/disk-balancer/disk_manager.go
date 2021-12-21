package disk_balancer

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/disk"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	. "github.com/0chain/blobber/code/go/0chain.net/core/logging"
)

type (
	diskTier struct {
		checkTimeOut     time.Duration
		minDiskSize      uint64
		mountPoint       string
		partitions       []string
		selectNextVolume func(fileSize int64, volumes []*volume) (string, error)
		volumes          []*volume
	}

	volume struct {
		availableSize uint64
		path          string
	}
)

const (
	// MinSizeFirst the strategy of choosing the directory with the least occupied space.
	MinSizeFirst = "min_size_first"
)

// checkDisks checks for sufficient space and write-ability in the listed directories.
func (d *diskTier) checkDisks() error {
	var dParts []string
	disks, _ := disk.Partitions(false)
	ref := regexp.MustCompile(d.mountPoint)
	for _, disk := range disks {
		if ref.MatchString(disk.Mountpoint) {
			if !d.canUsed(disk.Mountpoint) {
				continue
			}
			dParts = append(dParts, disk.Mountpoint)
		}
	}

	d.partitions = dParts
	d.updateVolumes()

	if len(d.volumes) == 0 {
		Logger.Error("checkDisks(): no volumes for storage data")
		return errors.New("checkDisks(): no volumes for storage data")
	}

	return nil
}

// updateVolumes updated list volumes
func (d *diskTier) updateVolumes() {
	var volumes []*volume
	for _, p := range d.partitions {
		vol := &volume{path: p}
		if err := vol.getAvailableSize(); err != nil {
			Logger.Error(fmt.Sprintf("checkDisks() filed %v", err))
			continue
		}
		volumes = append(volumes, vol)
	}

	d.volumes = volumes
}

// GetNextVolumePath implemented DiskSelector interface.
func (d *diskTier) GetNextVolumePath(fileSize int64) (string, error) {
	return d.selectNextVolume(fileSize, d.volumes)
}

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

// init initializes the listed directories and registers the write strategy.
func (d *diskTier) init() error {
	d.minDiskSize = config.Configuration.MinDiskSize
	d.mountPoint = config.Configuration.MountPoint
	d.selectNextVolume = d.selectStrategy(config.Configuration.Strategy)

	if err := d.checkDisks(); err != nil {
		return err
	}

	timer := time.NewTimer(d.checkTimeOut)
	for {
		select {
		case <-timer.C:
			// TODO implement handle error
			_ = d.checkDisks()
		}
	}

	return nil
}

// selectStrategy registers a function for selecting directories for storage.
func (d *diskTier) selectStrategy(strategy string) func(fileSize int64, volumes []*volume) (string, error) {
	switch strategy {
	case MinSizeFirst:
		return func(fileSize int64, volumes []*volume) (string, error) {
			minSize := volumes[0].availableSize
			violInd := 0
			for i, v := range volumes {
				if err := v.getAvailableSize(); err != nil {
					continue
				}
				if v.availableSize > minSize {
					minSize = v.availableSize
					violInd = i
				}
			}

			if !volumes[violInd].isAbleToStoreData(fileSize) {
				return "", errors.New("available size for volume is less than fileSize to store")
			}

			return volumes[violInd].path, nil
		}
	default:
		return nil
	}
}

// getAvailableSize gets information about the state of a directory.
func (v *volume) getAvailableSize() error {
	var volStat unix.Statfs_t
	err := unix.Statfs(v.path, &volStat)
	if err != nil {
		Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return err
	}

	v.availableSize = volStat.Bfree * uint64(volStat.Bsize)

	return nil
}

// isAbleToStoreData checks the ability to write to the directory.
func (v *volume) isAbleToStoreData(fileSize int64) (ableToStore bool) {
	if v.availableSize < uint64(fileSize) {
		Logger.Error(fmt.Sprintf("Available size for volume %v is less than fileSize to store (%v)", v.path, fileSize))
		return
	}

	if unix.Access(v.path, unix.W_OK) != nil {
		return
	}

	return true
}
