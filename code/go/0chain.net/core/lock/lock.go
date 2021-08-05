package lock

import (
	"sync"
	"time"
)

var (
	lockPool   = make(map[string]*Mutex)
	unlockPool = make(map[string]time.Time)
	lockMutex  sync.Mutex
)

// Mutex a mutual exclusion lock.
type Mutex struct {
	// key lock key in pool
	key string
	sync.Mutex
}

// Lock implements Locker.Lock
func (m *Mutex) Lock() {
	m.Mutex.Lock()
}

// Unlock implements Locker.Unlock, and mark mutex as unlock object
func (m *Mutex) Unlock() {
	lockMutex.Lock()
	defer lockMutex.Unlock()

	m.Mutex.Unlock()
	//mark it as unlock object, it will be deleted in clean worker
	unlockPool[m.key] = time.Now()
}

// GetMutex get mutex by table and key
func GetMutex(tablename string, key string) *Mutex {
	lockKey := tablename + ":" + key
	lockMutex.Lock()
	defer lockMutex.Unlock()
	if eLock, ok := lockPool[lockKey]; ok {
		// do NOT remove it from pool
		delete(unlockPool, lockKey)
		return eLock
	}

	m := &Mutex{key: lockKey}
	lockPool[lockKey] = m

	return m
}

func init() {
	go startLockPoolCleaner()
}

func startLockPoolCleaner() {
	for {
		time.Sleep(1 * time.Hour)

		lockMutex.Lock()

		for key := range unlockPool {
			delete(lockPool, key)
		}

		unlockPool = make(map[string]time.Time)
		lockMutex.Unlock()
	}
}
