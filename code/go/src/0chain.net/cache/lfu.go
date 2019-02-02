package cache

import (
	"github.com/koding/cache"
)

type LFU struct {
	Cache cache.Cache
}

//NewLFUCache - create a new LFU cache object
func NewLFUCache(size int) *LFU {
	c := &LFU{}
	c.Cache = cache.NewLFU(size)
	return c
}

//Add - add a given key and value
func (c *LFU) Add(key string, value interface{}) error {
	return c.Cache.Set(key, value)
}

//Get - get the value associated with the key
func (c *LFU) Get(key string) (interface{}, error) {
	value, err := c.Cache.Get(key)
	if err != nil {
		return nil, err
	}
	return value, err
}

func (c *LFU) Delete(key string) error {
	return c.Cache.Delete(key)
}
