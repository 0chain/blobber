package challenge

import (
	"sync"
)

var cMap = &ChallengeMap{
	items: make(map[string]ChallengeStatus),
}

type ChallengeMap struct {
	sync.RWMutex
	items map[string]ChallengeStatus
}

func (f *ChallengeMap) Exists(id string) (ChallengeStatus, bool) {
	f.RLock()
	defer f.RUnlock()

	s, ok := f.items[id]

	return s, ok

}

func (f *ChallengeMap) Add(id string, status ChallengeStatus) {
	f.Lock()
	defer f.Unlock()

	f.items[id] = status
}

func (f *ChallengeMap) Remove(id string) {
	f.Lock()
	defer f.Unlock()

	delete(f.items, id)
}

func (f *ChallengeMap) Count() int {
	f.RLock()
	defer f.RUnlock()

	return len(f.items)
}
