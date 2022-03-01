package writemarker

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/datastore"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
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
	sync.Mutex
}

// Lock
func (m *Mutex) Lock(ctx context.Context, allocationID, connectionID string, requestTime *time.Time) (*LockResult, error) {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	if len(allocationID) == 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "allocationID")
	}

	if len(connectionID) == 0 {
		return nil, errors.Throw(constants.ErrInvalidParameter, "connectionID")
	}

	if requestTime == nil {
		return nil, errors.Throw(constants.ErrInvalidParameter, "requestTime")
	}

	now := time.Now()
	if requestTime.After(now.Add(config.Configuration.WriteMarkerLockTimeout)) {
		return nil, errors.Throw(constants.ErrInvalidParameter, "requestTime")
	}

	db := datastore.GetStore().GetDB()

	var lock datastore.WriteLock
	err := db.Table(datastore.TableNameWriteLock).Where("allocation_id=?", allocationID).First(&lock).Error
	if err != nil {
		// new lock
		if errors.Is(err, gorm.ErrRecordNotFound) {
			lock = datastore.WriteLock{
				AllocationID: allocationID,
				SessionID:    connectionID,
				CreatedAt:    *requestTime,
			}

			err = db.Create(&lock).Error
			if err != nil {
				return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
			}

			return &LockResult{
				Status:    LockStatusOK,
				CreatedAt: lock.CreatedAt.Unix(),
			}, nil

		}

		//native postgres error
		return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)

	}

	timeout := lock.CreatedAt.Add(config.Configuration.WriteMarkerLockTimeout)

	// locked, but it is timeout
	if now.After(timeout) {

		lock.SessionID = connectionID
		lock.CreatedAt = *requestTime

		err = db.Save(&lock).Error
		if err != nil {
			return nil, errors.ThrowLog(err.Error(), common.ErrBadDataStore)
		}

		return &LockResult{
			Status:    LockStatusOK,
			CreatedAt: lock.CreatedAt.Unix(),
		}, nil

	}

	//try lock by same session, return old lock directly
	if lock.SessionID == connectionID && lock.CreatedAt.Equal(*requestTime) {
		return &LockResult{
			Status:    LockStatusOK,
			CreatedAt: lock.CreatedAt.Unix(),
		}, nil
	}

	// pending
	return &LockResult{
		Status:    LockStatusPending,
		CreatedAt: lock.CreatedAt.Unix(),
	}, nil

}

func (*Mutex) Unlock(ctx context.Context, allocationID string, sessionID string) error {

	if len(allocationID) == 0 {
		return nil
	}

	if len(sessionID) == 0 {
		return nil
	}

	db := datastore.GetStore().GetDB()

	err := db.Where("allocation_id = ? and session_id = ? ", allocationID, sessionID).Delete(&datastore.WriteLock{}).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return errors.ThrowLog(err.Error(), common.ErrBadDataStore)
	}

	return nil
}
