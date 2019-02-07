package lock

import "sync"

var lockPool = make(map[string]*sync.Mutex, 0)
var lockMutex = &sync.Mutex{}

func GetMutex(key string) *sync.Mutex {
	lockMutex.Lock()
	defer lockMutex.Unlock()
	if eLock, ok := lockPool[key]; ok {
		return eLock
	}
	lockPool[key] = &sync.Mutex{}
	return lockPool[key]
}
