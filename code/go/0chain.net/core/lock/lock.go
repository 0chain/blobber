package lock

import (
	"sync"
	"time"
)

var (
	// MutexCleanInterval start to clean unused mutex at specified interval
	MutexCleanInterval = 10 * time.Minute
)

var (
	lockPool  = make(map[string]*Mutex)
	lockMutex sync.Mutex
)

// Mutex a mutual exclusion lock.
type Mutex struct {
	// usedby how objects it is used by
	usedby int

	mu *sync.RWMutex
}

// Lock implements Locker.Lock
func (m *Mutex) Lock() {
	m.mu.Lock()
}

func (m *Mutex) RLock() {
	m.mu.RLock()
}

func (m *Mutex) RUnlock() {
	m.mu.RUnlock()
}

// Unlock implements Locker.Unlock, and mark mutex as unlock object
func (m *Mutex) Unlock() {
	lockMutex.Lock()
	defer lockMutex.Unlock()

	m.usedby--
	m.mu.Unlock()
}

// GetMutex get mutex by table and key
func GetMutex(tablename, key string) *Mutex {
	lockKey := tablename + ":" + key
	lockMutex.Lock()

	defer lockMutex.Unlock()
	if eLock, ok := lockPool[lockKey]; ok {
		eLock.usedby++
		return eLock
	}

	m := &Mutex{
		usedby: 1,
		mu:     &sync.RWMutex{},
	}

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
