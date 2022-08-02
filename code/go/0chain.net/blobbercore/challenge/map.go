package challenge

import (
	"sync"

	"github.com/0chain/blobber/code/go/0chain.net/core/cache"
)

var cMap = &ChallengeMap{
	items: cache.NewLRUCache(10000),
}

type ChallengeMap struct {
	sync.RWMutex
	count int
	items *cache.LRU
}

func (f *ChallengeMap) Exists(id string) (*ChallengeStatus, bool) {
	f.RLock()
	defer f.RUnlock()

	s, err := f.items.Get(id)
	if err != nil {
		return nil, false
	}

	return s.(*ChallengeStatus), true

}

func (f *ChallengeMap) Add(id string, status ChallengeStatus) {
	f.Lock()
	defer f.Unlock()

	f.items.Add(id, status) //nolint: errcheck
	f.count++
}

func (f *ChallengeMap) Remove(id string) {
	f.Lock()
	defer f.Unlock()

	f.items.Delete(id) //nolint: errcheck
	f.count--
}

func (f *ChallengeMap) Count() int {
	f.RLock()
	defer f.RUnlock()

	return f.count
}
