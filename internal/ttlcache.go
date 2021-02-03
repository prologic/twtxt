package internal

import (
	"sync"
	"time"
)

type cachedItem struct {
	value  int
	expiry time.Time
}

func (item cachedItem) expired() bool {
	return time.Now().After(item.expiry)
}

type cachedItems map[string]cachedItem

type TTLCache struct {
	sync.RWMutex

	ttl   time.Duration
	items map[string]cachedItem
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
	return v.value
}

func (cache *TTLCache) Set(k string, v int) int {
	cache.Lock()
	defer cache.Unlock()

	cache.items[k] = cachedItem{v, time.Now().Add(cache.ttl)}

	return v
}

func (cache *TTLCache) Reset(k string) int {
	return cache.Set(k, 0)
}

func NewTTLCache(ttl time.Duration) *TTLCache {
	cache := &TTLCache{ttl: ttl, items: make(cachedItems)}

	go func() {
		for range time.Tick(ttl) {
			cache.Lock()
			for k, v := range cache.items {
				if v.expired() {
					delete(cache.items, k)
				}
			}
			cache.Unlock()
		}
	}()

	return cache
}
