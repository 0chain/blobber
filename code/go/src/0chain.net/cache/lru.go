package cache

import (
	lru "github.com/koding/cache"
)

//LRU - LRU cache
type LRU struct {
	Cache lru.Cache
}

//NewLRUCache - create a new LRU cache
func NewLRUCache(size int) *LRU {
	c := &LRU{}
	c.Cache = lru.NewLRU(size)
	return c
}

//Add - add a key and a value
func (c *LRU) Add(key string, value interface{}) error {
	c.Cache.Set(key, value)
	return nil
}

//Get - get the value associated with the key
func (c *LRU) Get(key string) (interface{}, error) {
	value, err := c.Cache.Get(key)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (c *LRU) Delete(key string) error {
	return c.Cache.Delete(key)
}
