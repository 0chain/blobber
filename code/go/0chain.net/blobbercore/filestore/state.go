package filestore

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"go.uber.org/zap"
)

type allocation struct {
	mu            *sync.Mutex
	allocatedSize uint64
	filesNumber   uint64
	filesSize     uint64

	tmpMU       *sync.Mutex
	tmpFileSize uint64
}

func (fs *FileStore) setAllocation(allocID string, alloc *allocation) {
	fs.rwMU.Lock()
	defer fs.rwMU.Unlock()
	fs.mAllocs[allocID] = alloc
}

func (fs *FileStore) getAllocation(allocID string) *allocation {
	fs.rwMU.RLock()
	defer fs.rwMU.RUnlock()
	return fs.mAllocs[allocID]
}

func (fs *FileStore) removeAllocation(ID string) {
	fs.rwMU.Lock()
	defer fs.rwMU.Unlock()
	delete(fs.mAllocs, ID)
}

func (fs *FileStore) initMap() error {
	ctx, cnCl := context.WithCancel(context.Background())
	defer cnCl()

	ctx = datastore.GetStore().CreateTransaction(ctx)
	db := datastore.GetStore().GetTransaction(ctx)
	if db == nil {
		return errors.New("could not get db client")
	}

	var allocations []*dbAllocation
	if err := db.Model(&dbAllocation{}).Find(&allocations).Error; err != nil {
		return err
	}

	limitCh := make(chan struct{}, 50)
	wg := &sync.WaitGroup{}

	for _, alloc := range allocations {
		a := allocation{
			allocatedSize: uint64(alloc.BlobberSize),
			mu:            &sync.Mutex{},
			tmpMU:         &sync.Mutex{},
		}

		fs.setAllocation(alloc.ID, &a)

		err := getStorageDetails(ctx, &a, alloc.ID)

		if err != nil {
			return err
		}

		limitCh <- struct{}{}
		wg.Add(1)
		go fs.getTemporaryStorageDetails(ctx, &a, alloc.ID, limitCh, wg)

	}

	wg.Wait()
	db.Commit()
	return nil
}

func (fs *FileStore) incrDecrAllocFileSizeAndNumber(allocID string, size int64, fileNumber int64) {
	alloc := fs.mAllocs[allocID]
	if alloc == nil {
		logging.Logger.Debug("alloc is nil", zap.String("allocation_id", allocID))
		return
	}

	alloc.mu.Lock()
	defer alloc.mu.Unlock()

	alloc.filesSize += uint64(size)
	alloc.filesNumber += uint64(fileNumber)
}

func (fs *FileStore) GetDiskUsedByAllocation(allocID string) uint64 {
	alloc := fs.mAllocs[allocID]
	if alloc != nil {
		return alloc.filesSize + alloc.tmpFileSize
	}
	return 0
}

func (fs *FileStore) GetDiskUsedByAllocations() (s uint64) {
	for _, alloc := range fs.mAllocs {
		s += alloc.filesSize + alloc.tmpFileSize
	}
	return
}

type dbAllocation struct {
	ID              string           `gorm:"column:id"`
	Expiration      common.Timestamp `gorm:"column:expiration_date"`
	BlobberSize     int64            `gorm:"column:blobber_size"`
	BlobberSizeUsed int64            `gorm:"column:blobber_size_used"`
	TimeUnit        time.Duration    `gorm:"column:time_unit"`

	// Ending and cleaning
	CleanedUp bool `gorm:"column:cleaned_up"`
	Finalized bool `gorm:"column:finalized"`
}

func (dbAllocation) TableName() string {
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

func getStorageDetails(ctx context.Context, a *allocation, ID string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	r := map[string]interface{}{
		"allocation_id": ID,
		"type":          "f",
		"on_cloud":      false,
	}
	var totalFiles int64
	if err := db.Model(&ref{}).Where(r).Count(&totalFiles).Error; err != nil {
		return err
	}

	var totalFileSize *int64
	if err := db.Model(&ref{}).Select("sum(size) as file_size").Where(r).Scan(&totalFileSize).Error; err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.filesNumber = uint64(totalFiles)
	if totalFileSize != nil {
		a.filesSize = uint64(*totalFileSize)
	}
	return nil
}

// UpdateAllocationMetaData only updates if allocation size has changed or new allocation is allocated. Must use allocationID.
// Use of allocation Tx might leak memory. allocation size must be of int64 type otherwise it won't be updated
func (fs *FileStore) UpdateAllocationMetaData(m map[string]interface{}) {
	fs.allocMu.Lock()
	defer fs.allocMu.Unlock()

	allocIDI := m["allocation_id"]
	if allocIDI == nil {
		return
	}

	allocID, ok := allocIDI.(string)
	if !ok {
		return
	}

	allocatedSizeI := m["allocated_size"]
	if allocatedSizeI == nil {
		return
	}

	allocatedSize, ok := allocatedSizeI.(int64)
	if !ok {
		return
	}
	alloc := fs.getAllocation(allocID)
	if alloc == nil {
		alloc = &allocation{
			allocatedSize: uint64(allocatedSize),
			mu:            &sync.Mutex{},
			tmpMU:         &sync.Mutex{},
		}

		fs.setAllocation(allocID, alloc)
		return
	}

	alloc.allocatedSize = uint64(allocatedSize)

}
