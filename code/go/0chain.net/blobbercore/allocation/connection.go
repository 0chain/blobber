package allocation

import (
	"context"
	"sync"
	"time"

	"github.com/0chain/blobber/code/go/0chain.net/blobbercore/filestore"
)

var (
	// ConnectionObjCleanInterval start to clean the connectionObjMap
	ConnectionObjCleanInterval = 10 * time.Minute
	// ConnectionObjTimout after which connectionObj entry should be invalid
	ConnectionObjTimeout = 10 * time.Minute
)

var (
	connectionObjSizeMap = make(map[string]*ConnectionObjSize)
	connectionObjMutex   sync.RWMutex
)

type ConnectionObjSize struct {
	Size      int64
	UpdatedAt time.Time
	Changes   map[string]*ConnectionChanges
}

type ConnectionChanges struct {
	Hasher *filestore.CommitHasher
}

// GetConnectionObjSize gets the connection size from the memory
func GetConnectionObjSize(connectionID string) int64 {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
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
			Changes:   make(map[string]*ConnectionChanges),
		}
		return
	}

	connectionObjSize.Size = connectionObjSize.Size + addSize
	connectionObjSize.UpdatedAt = time.Now()
}

func GetHasher(connectionID, pathHash string) *filestore.CommitHasher {
	connectionObjMutex.RLock()
	defer connectionObjMutex.RUnlock()
	connectionObj := connectionObjSizeMap[connectionID]
	if connectionObj == nil {
		return nil
	}
	if connectionObj.Changes[pathHash] == nil {
		return nil
	}
	return connectionObj.Changes[pathHash].Hasher
}

func UpdateConnectionObjWithHasher(connectionID, pathHash string, hasher *filestore.CommitHasher) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObj := connectionObjSizeMap[connectionID]
	if connectionObj == nil {
		connectionObjSizeMap[connectionID] = &ConnectionObjSize{
			UpdatedAt: time.Now(),
			Changes:   make(map[string]*ConnectionChanges),
		}
	}
	connectionObjSizeMap[connectionID].Changes[pathHash] = &ConnectionChanges{
		Hasher: hasher,
	}
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
