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

// TryLockWithTimeout attempts to acquire the write lock, polling until the
// timeout elapses. Returns true if acquired (caller must Unlock), false on
// timeout (caller must call GiveBack to balance the GetMutex reference). Lets
// the commit path bound how long it waits on the per-allocation lock instead
// of blocking indefinitely behind a long-running orphan-GC / challenge.
func (m *Mutex) TryLockWithTimeout(d time.Duration) bool {
	if d <= 0 {
		m.mu.Lock()
		return true
	}
	deadline := time.Now().Add(d)
	for {
		if m.mu.TryLock() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// GiveBack releases the GetMutex reference WITHOUT unlocking — used when
// TryLockWithTimeout times out. Keeps usedby balanced with GetMutex.
func (m *Mutex) GiveBack() {
	lockMutex.Lock()
	defer lockMutex.Unlock()
	m.usedby--
}

func (m *Mutex) RLock() {
	m.mu.RLock()
}

func (m *Mutex) RUnlock() {
	lockMutex.Lock()
	defer lockMutex.Unlock()
	m.usedby--
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
