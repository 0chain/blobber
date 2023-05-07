package allocation

import (
	"context"
	"sync"
	"time"
)

var (
	// ConnectionObjCleanInterval start to clean the connectionObjMap
	ConnectionObjCleanInterval = 10 * time.Minute
	// ConnectionObjTimout after which connectionObj entry should be invalid
	ConnectionObjTimeout = 10 * time.Minute
)

var (
	connectionObjSizeMap = make(map[string]*ConnectionObjSize)
	connectionObjMutex   sync.Mutex
)

type ConnectionObjSize struct {
	Size      int64
	UpdatedAt time.Time
}

// GetConnectionObjSize gets the connection size from the memory
func GetConnectionObjSize(connectionID string) int64 {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObjSize := connectionObjSizeMap[connectionID]
	if connectionObjSize == nil {
		return 0
	}
	return connectionObjSizeMap[connectionID].Size
}

// UpdateConnectionObjSize updates the connection size by addSize in memory
func UpdateConnectionObjSize(connectionID string, addSize int64) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObjSize := connectionObjSizeMap[connectionID]
	if connectionObjSize == nil {
		connectionObjSizeMap[connectionID] = &ConnectionObjSize{
			Size:      addSize,
			UpdatedAt: time.Now(),
		}
		return
	}

	connectionObjSize.Size = connectionObjSize.Size + addSize
	connectionObjSize.UpdatedAt = time.Now()
}

// DeleteConnectionObjEntry remove the connectionID entry from map
// If the given connectionID is not present, then it is no-op.
func DeleteConnectionObjEntry(connectionID string) {
	connectionObjMutex.Lock()
	delete(connectionObjSizeMap, connectionID)
	connectionObjMutex.Unlock()
}

// cleanConnectionObj cleans the connectionObjSize map. It deletes the rows
// for which deadline is exceeded.
func cleanConnectionObj() {
	connectionObjMutex.Lock()
	for connectionID, connectionObjSize := range connectionObjSizeMap {
		diff := time.Since(connectionObjSize.UpdatedAt)
		if diff >= ConnectionObjTimeout {
			delete(connectionObjSizeMap, connectionID)
		}
	}
	connectionObjMutex.Unlock()
}

func startCleanConnectionObj(ctx context.Context) {
	ticker := time.NewTicker(ConnectionObjCleanInterval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanConnectionObj()
		}

	}
}

func SetupWorkers(ctx context.Context) {
	go startCleanConnectionObj(ctx)
}
