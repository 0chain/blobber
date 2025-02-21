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
	"gorm.io/gorm"
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
	fs.mAllocs[allocID] = alloc
	fs.rwMU.Unlock()
}

func (fs *FileStore) setAllocations(m map[string]*allocation) {
	fs.rwMU.Lock()
	for allocID, alloc := range m {
		fs.mAllocs[allocID] = alloc
	}
	fs.rwMU.Unlock()
}

func (fs *FileStore) getAllocation(allocID string) *allocation {
	fs.rwMU.RLock()
	alloc := fs.mAllocs[allocID]
	fs.rwMU.RUnlock()
	return alloc
}

func (fs *FileStore) removeAllocation(ID string) {
	fs.rwMU.Lock()
	delete(fs.mAllocs, ID)
	fs.rwMU.Unlock()
}

func (fs *FileStore) initMap() error {
	err := datastore.GetStore().WithNewTransaction(func(ctx context.Context) error {
		db := datastore.GetStore().GetTransaction(ctx)
		if db == nil {
			return errors.New("could not get db client")
		}

		var dbAllocations []*dbAllocation

		err := db.Model(&dbAllocation{}).FindInBatches(&dbAllocations, 1000, func(tx *gorm.DB, batch int) error {
			allocsMap := make(map[string]*allocation)

			for _, dbAlloc := range dbAllocations {
				a := allocation{
					allocatedSize: uint64(dbAlloc.BlobberSize),
					mu:            &sync.Mutex{},
					tmpMU:         &sync.Mutex{},
				}

				allocsMap[dbAlloc.ID] = &a

				err := getStorageDetails(ctx, &a, dbAlloc.ID)

				if err != nil {
					return err
				}
			}

			fs.setAllocations(allocsMap)
			return nil
		}).Error

		return err
	})
	return err
}

func (fs *FileStore) incrDecrAllocFileSizeAndNumber(allocID string, size int64, fileNumber int64) {
	alloc := fs.getAllocation(allocID)
	if alloc == nil {
		logging.Logger.Debug("alloc is nil", zap.String("allocation_id", allocID))
		return
	}

	alloc.mu.Lock()

	alloc.filesSize += uint64(size)
	alloc.filesNumber += uint64(fileNumber)

	alloc.mu.Unlock()
}

func (fs *FileStore) GetDiskUsedByAllocation(allocID string) uint64 {
	alloc := fs.getAllocation(allocID)
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
}

func (ref) TableName() string {
	return "reference_objects"
}

func getStorageDetails(ctx context.Context, a *allocation, ID string) error {
	db := datastore.GetStore().GetTransaction(ctx)
	r := map[string]interface{}{
		"allocation_id": ID,
		"type":          "f",
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
func (fs *FileStore) UpdateAllocationMetaData(m map[string]interface{}) error {
	fs.allocMu.Lock()
	defer fs.allocMu.Unlock()

	allocIDI := m["allocation_id"]
	if allocIDI == nil {
		return errors.New("empty allocation id")
	}

	allocID, ok := allocIDI.(string)
	if !ok {
		return errors.New("allocation id is not string type")
	}

	allocatedSizeI := m["allocated_size"]
	if allocatedSizeI == nil {
		return errors.New("empty allocated size value")
	}

	allocatedSize, ok := allocatedSizeI.(int64)
	if !ok {
		return errors.New("allocated size is not int64 type")
	}
	alloc := fs.getAllocation(allocID)
	if alloc == nil {
		alloc = &allocation{
			allocatedSize: uint64(allocatedSize),
			mu:            &sync.Mutex{},
			tmpMU:         &sync.Mutex{},
		}

		fs.setAllocation(allocID, alloc)
		return nil
	}

	alloc.allocatedSize = uint64(allocatedSize)
	return nil
}
