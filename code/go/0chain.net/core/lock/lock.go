package lock

import (
	"sync"
	"time"
)

// MutexCleanInterval start to clean unused mutex at specified interval
var MutexCleanInterval = 10 * time.Minute

var (
	lockPool  = make(map[string]*Mutex)
	lockMutex sync.Mutex
)

// Mutex a mutual exclusion lock.
type Mutex struct {
	// key lock key in pool
	key string
	// usedby how objects it is used by
	usedby int

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

	m.usedby--
	m.Mutex.Unlock()
}

// GetMutex get mutex by table and key
func GetMutex(tablename string, key string) *Mutex {
	lockKey := tablename + ":" + key
	lockMutex.Lock()

	defer lockMutex.Unlock()
	if eLock, ok := lockPool[lockKey]; ok {
		eLock.usedby++
		return eLock
	}

	m := &Mutex{key: lockKey, usedby: 1}

	lockPool[lockKey] = m

	return m
}

func init() {
	go startWorker()
}

func cleanUnusedMutexs() {
	lockMutex.Lock()

	for k, v := range lockPool {
		if v.usedby < 1 {
			delete(lockPool, k)
		}
	}

	lockMutex.Unlock()
}

func startWorker() {
	for {
		time.Sleep(MutexCleanInterval)

		cleanUnusedMutexs()

	}
}
