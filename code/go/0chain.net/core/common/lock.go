package common

import (
	"sync"
)

type Lock struct {
	key string
	// stale signals if lock is deleted from map. if true, discard use of this lock and get fresh lock
	stale      bool
	actualLock *sync.Mutex
	// count how many process requires this lock
	count   int32
	countMu *sync.Mutex

	// Parent Map
	pMap *MapLocker

	tryUnlockMu *sync.Mutex
}

// Lock Acquire lock
func (l *Lock) Lock() {
	l.countMu.Lock()
	l.count++
	l.countMu.Unlock()
	for {
		l.actualLock.Lock()
		if l.stale {
			newLock, _ := l.pMap.GetLock(l.key)
			*l = *newLock // nolint // Its safe as it copies address of lock
			continue
		}
		break
	}
}

func (l *Lock) Unlock() {
	l.actualLock.Unlock()
	l.countMu.Lock()
	l.count--
	if l.count == 0 {
		l.stale = true
		l.pMap.m.Delete(l.key)
	}
	l.countMu.Unlock()
}

// TryUnlock unlock the lock if it is already locked otherwise do nothing.
// Don't use tryUnlock and Unlock together for the same lock otherwise it may panic
func (l *Lock) TryUnlock() {
	l.tryUnlockMu.Lock()
	defer l.tryUnlockMu.Unlock()
	if l.actualLock.TryLock() {
		// If succeed then unlock it safely
		l.Unlock()
	} else {
		// The lock is already acquired by other process, and tryUnlockMu make sure that
		// lock is not unlocked by other TryUnlock().
		l.Unlock()
	}
}

type MapLocker struct {
	m *sync.Map
	// Reduce updating lock with GetLock call with same key
	mu *sync.Mutex
}

func (m *MapLocker) GetLock(key string) (l *Lock, isNew bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	valueI, ok := m.m.Load(key)
	if ok {
		l = valueI.(*Lock)
		return
	}

	l = &Lock{
		key:         key,
		pMap:        m,
		actualLock:  new(sync.Mutex),
		countMu:     new(sync.Mutex),
		tryUnlockMu: new(sync.Mutex),
	}
	isNew = true
	m.m.Store(key, l)
	return
}

func GetNewLocker() *MapLocker {
	return &MapLocker{
		mu: new(sync.Mutex),
		m:  new(sync.Map),
	}
}
