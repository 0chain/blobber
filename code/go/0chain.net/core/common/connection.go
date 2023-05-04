package common

import (
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
	connectionObjSizeMap = make(map[string]ConnectionObjSize)
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
	connectionObjSize, ok := connectionObjSizeMap[connectionID]
	if !ok {
		return 0
	}
	return connectionObjSize.Size
}

// UpdateConnectionObjSize updates the connection size by addSize in memory 
func UpdateConnectionObjSize(connectionID string, addSize int64) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	connectionObjSize, _ := connectionObjSizeMap[connectionID];
	connectionObjSizeMap[connectionID] = ConnectionObjSize{Size: connectionObjSize.Size + addSize, UpdatedAt: time.Now()}
}

// DeleteConnectionObjEntry remove the connectionID entry from map
// If the given connectionID is not present, then it is no-op. 
func DeleteConnectionObjEntry(connectionID string) {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	delete(connectionObjSizeMap, connectionID)
}

// cleanConnectionObj cleans the connectionObjSize map. It deletes the rows
// for which deadline is exceeded.
func cleanConnectionObj() {
	connectionObjMutex.Lock()
	defer connectionObjMutex.Unlock()
	for connectionID, connectionObjSize := range connectionObjSizeMap {
		diff := time.Now().Sub(connectionObjSize.UpdatedAt)
		if diff >= ConnectionObjTimeout {
			delete(connectionObjSizeMap, connectionID)
		}
	}
}

func init() {
	go func() {
		for {
			time.Sleep(ConnectionObjCleanInterval)
			cleanConnectionObj()
		}
	}()
}
