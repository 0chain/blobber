package filestore

// File management i.e. evenly distributing files so that OS takes less time to process lookups, is tricky. One might think
// of creating multiple indexes, 0.......unlimited, but this will result in unlimited number of directories is base path.
//
// base path --> where you'd want to store all the files with some file management techniques.
//
// Using multiple indexes is manageable if allocation size would be fixed during its life time, but its not. It can decrease and
// increase. Also the blobber's size can increase or decrease. Thus, managing files using numerical indexes will be complex.
//
// To deal with it, we can use contentHash of some file so that files are distributed randomly. Randomness seems to distribute files
// close to evenly. So if we have an lookup hash of a file "4c9bad252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56" then we
// can store this file in following directory:
// `base_path/{allocation_id}/4/c/9/b/a/d252272bc6e3969be637610d58f3ab2ff8ca336ea2fadd6171fc68fdd56`
// With above structure, an allocation can have upto 16*16*16*16*16 = 1048576 directories for storing files and 16 + 16^2+16^3+16^4 = 69904
// for parent directories of 1048576 directories.
//
// If some direcotry would contain 1000 files then total files stored by an allocation would be 1048576*1000 = 1048576000, around 1 billions
// file.
// Blobber should choose this level calculating its size and number of files its going to store and increase/decrease levels of directories.
//
// Above situation also occurs to store {allocation_id} as base directory for its files when blobber can have thousands of allocations.
// We can also use allocation_id to distribute allocations.
// For allocation using 3 levels we would have 16*16*16 = 4096 unique directories, Each directory can contain 1000 allocations. Thus storing
// 4096000 number of allocations.
//
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"golang.org/x/sys/unix"
)

const (
	MaxFilesInADir   = 1000
	SmallestFileSize = 64 * 1024
)

var currentDiskCapacity uint64
var getMountPoint func() string

type allocation struct {
	mu            *sync.Mutex
	allocatedSize uint64
	filesNumber   uint64
	filesSize     uint64
}

type fileManager struct {
	Allocations map[string]*allocation
}

var fm fileManager

func InitManager(ctx context.Context, mp string) error {
	finfo, err := os.Stat(mp)
	if err != nil {
		return err
	}
	if !finfo.IsDir() {
		return errors.New("mount point is not directory type")
	}

	if err := validateLevels(); err != nil {
		return err
	}

	getMountPoint = func() string { return mp }

	db := datastore.GetStore().GetTransaction(ctx)
	if db == nil {
		return errors.New("could not get db client")
	}

	var allocations []*dbAlloc
	if err := db.Model(&dbAlloc{}).Find(&allocations).Error; err != nil {
		return err
	}

	fm = fileManager{
		Allocations: make(map[string]*allocation),
	}

	for _, alloc := range allocations {
		a := allocation{
			allocatedSize: uint64(alloc.BlobberSize),
			mu:            &sync.Mutex{},
		}

		fm.Allocations[alloc.ID] = &a

		err := getStorageDetails(ctx, &a, alloc.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func getStorageDetails(ctx context.Context, a *allocation, ID string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	r := map[string]interface{}{
		"allocation_id": ID,
		"type":          "f",
		"on_cloud":      false,
	}
	var totalFiles, totalFileSize int64
	if err := db.Model(&ref{}).Where(r).Count(&totalFiles).Error; err != nil {
		return err
	}

	if err := db.Model(&ref{}).Select("sum(size) as file_size").Where(r).Scan(&totalFileSize).Error; err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.filesNumber = uint64(totalFiles)
	a.filesSize = uint64(totalFileSize)
	return nil
}

type dbAlloc struct {
	ID              string           `gorm:"column:id"`
	Expiration      common.Timestamp `gorm:"column:expiration_date"`
	BlobberSize     int64            `gorm:"column:blobber_size"`
	BlobberSizeUsed int64            `gorm:"column:blobber_size_used"`
	TimeUnit        time.Duration    `gorm:"column:time_unit"`

	// Ending and cleaning
	CleanedUp bool `gorm:"column:cleaned_up"`
	Finalized bool `gorm:"column:finalized"`
}

func (dbAlloc) TableName() string {
	return "allocations"
}

type ref struct {
	Type         string `gorm:"column:type"`
	AllocationID string `gorm:"column:allocation_id"`
	Size         int64  `gorm:"column:size"`
	OnCloud      bool   `gorm:"column:on_cloud"`
}

func (ref) TableName() string {
	return "reference_objects"
}

func GetPathForFile(allocID, contentHash string) (string, error) {
	if len(allocID) != 64 || len(contentHash) != 64 {
		return "", errors.New("length of allocationID/contentHash must be 64")
	}

	allocDir := filepath.Join(getMountPoint(), getPath(allocID, getDirLevelsForAllocations()))
	return filepath.Join(allocDir, getPath(contentHash, getDirLevelsForFiles())), nil
}

func getDirLevelsForFiles() []int {
	return []int{2, 2, 1}
}

// getPath returns "/" separated strings with the given levels.
// Assumption is length of hash is 64
func getPath(hash string, levels []int) string {
	var count int
	var pStr []string
	for _, i := range levels {
		pStr = append(pStr, hash[count:count+i])
		count += i
	}
	pStr = append(pStr, hash[count:])
	return strings.Join(pStr, "/")
}

func getDirLevelsForAllocations() []int {
	return []int{2, 1}
}

// validateLevels will validate sum of levels which should not be greater than or equal to 64.
// It will be useful especially when levels are read from config files
func validateLevels() error {
	allocDirLevels := getDirLevelsForAllocations()
	var s int
	for _, i := range allocDirLevels {
		s += i
	}
	if s >= 64 {
		return errors.New("allocation directory levels has sum greater than 64")
	}

	s = 0
	fileDirLevels := getDirLevelsForFiles()
	for _, i := range fileDirLevels {
		s += i
	}
	if s >= 64 {
		return errors.New("files directory levels has sum greater than 64")
	}

	return nil
}

func GetCurrentDiskCapacity() uint64 {
	return currentDiskCapacity
}

func CalculateCurrentDiskCapacity() error {
	mp := getMountPoint()

	var volStat unix.Statfs_t
	err := unix.Statfs(mp, &volStat)
	if err != nil {
		logging.Logger.Error(fmt.Sprintf("getAvailableSize() unix.Statfs %v", err))
		return err
	}

	currentDiskCapacity = volStat.Bavail * uint64(volStat.Bsize)
	return nil
}
