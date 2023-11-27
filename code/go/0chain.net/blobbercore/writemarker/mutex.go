package writemarker

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/config"
	"github.com/0chain/blobber/code/go/0chain.net/core/common"
	"github.com/0chain/blobber/code/go/0chain.net/core/logging"
	"github.com/0chain/errors"
	"github.com/0chain/gosdk/constants"
	"go.uber.org/zap"
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

var (
	lockPool  = make(map[string]*WriteLock)
	lockMutex sync.Mutex
)

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
	lockMutex.Lock()
	defer lockMutex.Unlock()
	lock, ok := lockPool[allocationID]
	if !ok {
		// new lock
		logging.Logger.Info("Creating new lock")
		lock = &WriteLock{
			CreatedAt:    time.Now(),
			ConnectionID: connectionID,
		}
		lockPool[allocationID] = lock
		return &LockResult{
			Status:    LockStatusOK,
			CreatedAt: lock.CreatedAt.Unix(),
		}, nil
	}

	if lock.ConnectionID != connectionID {
		if time.Since(lock.CreatedAt) > config.Configuration.WriteMarkerLockTimeout || lock.ConnectionID == "" {
			// Lock expired. Provide lock to other connection id
			lock.ConnectionID = connectionID
			lock.CreatedAt = time.Now()
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

	return &LockResult{
		Status:    LockStatusOK,
		CreatedAt: lock.CreatedAt.Unix(),
	}, nil
}

func (*Mutex) Unlock(ctx context.Context, allocationID string, connectionID string) error {
	if allocationID == "" || connectionID == "" {
		return nil
	}
	lockMutex.Lock()
	defer lockMutex.Unlock()
	lock, ok := lockPool[allocationID]
	// reset lock if connection id matches
	if ok && lock.ConnectionID == connectionID {
		lock.ConnectionID = ""
	}

	return nil
}
