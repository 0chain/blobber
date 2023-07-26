package writemarker

import (
	"context"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// LockStatus lock status
type LockStatus int

const (
	LockStatusFailed LockStatus = iota
	LockStatusPending
	LockStatusOK
)

type LockResult struct {
	Status    LockStatus `json:"status,omitempty"`
	CreatedAt int64      `json:"created_at,omitempty"`
}

// Mutex WriteMarker mutex
type Mutex struct {
	// ML MapLocker
	ML *common.MapLocker
}

// Lock will create/update lock in postgres.
// If no lock exists for an allocation then new lock is created.
// If lock exists and is of same connection ID then lock's createdAt is updated
// If lock exists and is of other connection ID then `pending` response is sent.
func (m *Mutex) Lock(ctx context.Context, allocationID, connectionID string) (*LockResult, error) {
	logging.Logger.Info("Locking write marker", zap.String("allocation_id", allocationID), zap.String("connection_id", connectionID))
	if allocationID == "" {
		return nil, errors.Throw(constants.ErrInvalidParameter, "allocationID")
	}

	if connectionID == "" {
		return nil, errors.Throw(constants.ErrInvalidParameter, "connectionID")
	}

	l, _ := m.ML.GetLock(allocationID)
	l.Lock()
	defer l.Unlock()

	db := datastore.GetStore().GetDB()

	var lock WriteLock
	err := db.Table(TableNameWriteLock).Where("allocation_id=?", allocationID).First(&lock).Error
	if err != nil {
		// new lock
		logging.Logger.Info("Creating new lock")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			lock = WriteLock{
				AllocationID: allocationID,
				ConnectionID: connectionID,
				CreatedAt:    time.Now(),
			}

			err = db.Table(TableNameWriteLock).Create(&lock).Error
			if err != nil {
				return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
			}

			return &LockResult{
				Status:    LockStatusOK,
				CreatedAt: lock.CreatedAt.Unix(),
			}, nil
		}
		logging.Logger.Error("Could not create lock")
		//native postgres error
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	if lock.ConnectionID != connectionID {
		if time.Since(lock.CreatedAt) > config.Configuration.WriteMarkerLockTimeout {
			// Lock expired. Provide lock to other connection id
			lock.ConnectionID = connectionID
			lock.CreatedAt = time.Now()
			err = db.Model(&WriteLock{}).Where("allocation_id=?", allocationID).Save(&lock).Error
			if err != nil {
				return nil, errors.New("db_error", err.Error())
			}
			return &LockResult{
				Status:    LockStatusOK,
				CreatedAt: lock.CreatedAt.Unix(),
			}, nil
		}

		return &LockResult{
			Status:    LockStatusPending,
			CreatedAt: lock.CreatedAt.Unix(),
		}, nil
	}

	lock.CreatedAt = time.Now()
	err = db.Table(TableNameWriteLock).Where("allocation_id=?", allocationID).Save(&lock).Error
	if err != nil {
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	return &LockResult{
		Status:    LockStatusOK,
		CreatedAt: lock.CreatedAt.Unix(),
	}, nil
}

func (*Mutex) Unlock(ctx context.Context, allocationID string, connectionID string) error {
	if allocationID == "" || connectionID == "" {
		return nil
	}

	db := datastore.GetStore().GetDB()

	err := db.Exec("DELETE FROM write_locks WHERE allocation_id = ? and connection_id = ? ", allocationID, connectionID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	return nil
}
