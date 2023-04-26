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

	mu          *sync.Mutex
	tryUnlockMu *sync.Mutex
}

// Lock implements Locker.Lock
func (m *Mutex) Lock() {
	m.mu.Lock()
}

// Unlock implements Locker.Unlock, and mark mutex as unlock object
func (m *Mutex) Unlock() {
	lockMutex.Lock()
	defer lockMutex.Unlock()

	m.usedby--
	m.mu.Unlock()
}

// TryUnlock unlock the lock if it is already locked otherwise do nothing.
// Don't use tryUnlock and Unlock together for the same lock otherwise it may panic
func (m *Mutex) TryUnlock() {
	m.tryUnlockMu.Lock()
	defer m.tryUnlockMu.Unlock()
	if m.mu.TryLock() {
		// If succeed then unlock it safely
		m.mu.Unlock()
	} else {
		// The lock is already acquired by other process, and tryUnlockMu make sure that
		// lock is not unlocked by other TryUnlock().
		m.Unlock()
	}
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
		usedby:      1,
		mu:          &sync.Mutex{},
		tryUnlockMu: &sync.Mutex{},
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
