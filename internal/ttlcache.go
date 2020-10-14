package internal

import (
	"sync"
	"time"
)

type CachedItem struct {
	Value  int
	Expiry time.Time
}

func (item CachedItem) Expired() bool {
	return time.Now().After(item.Expiry)
}

type CachedItems map[string]CachedItem

type TTLCache struct {
	sync.RWMutex

	ttl   time.Duration
	items map[string]CachedItem
}

func (cache *TTLCache) Dec(k string) int {
	return cache.Set(k, cache.Get(k)-1)
}

func (cache *TTLCache) Inc(k string) int {
	return cache.Set(k, cache.Get(k)+1)
}

func (cache *TTLCache) Get(k string) int {
	cache.RLock()
	defer cache.RUnlock()
	v, ok := cache.items[k]
	if !ok {
		return 0
	}
	return v.Value
}

func (cache *TTLCache) Set(k string, v int) int {
	cache.Lock()
	defer cache.Unlock()

	cache.items[k] = CachedItem{v, time.Now().Add(cache.ttl)}

	return v
}

func (cache *TTLCache) Reset(k string) int {
	return cache.Set(k, 0)
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	cache := &TTLCache{ttl: ttl, items: make(CachedItems)}

	go func() {
		for range time.Tick(ttl) {
			cache.Lock()
			for k, v := range cache.items {
				if v.Expired() {
					delete(cache.items, k)
				}
			}
			cache.Unlock()
		}
	}()

	return cache
}
