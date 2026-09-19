package result

import (
	"seanime/internal/constants"
	"seanime/internal/util"
	"sync/atomic"
	"time"
)

type Cache[K interface{}, V any] struct {
	store util.RWMutexMap
	// sweeping guards the opportunistic sweep so only one runs at a time.
	sweeping atomic.Bool
	// writes counts Set calls since the last sweep.
	writes atomic.Int64
}

type cacheItem[K interface{}, V any] struct {
	value      V
	expiration time.Time
}

func NewCache[K interface{}, V any]() *Cache[K, V] {
	return &Cache[K, V]{}
}

// sweepEvery is how many writes pass between sweeps of expired entries.
const sweepEvery = 256

func (c *Cache[K, V]) Set(key K, value V) {
	c.SetT(key, value, constants.GcTime)
}

// SetT stores a value under key for ttl.
//
// Expiry is lazy — Get drops what it finds expired — with a periodic sweep to evict entries nobody
// reads again. Each Set used to spawn a goroutine that slept out the whole TTL instead: one live
// goroutine per cached entry, and re-setting a key left the previous timer running, so a value
// refreshed just before its TTL elapsed was deleted moments later by the timer of the value it
// replaced.
func (c *Cache[K, V]) SetT(key K, value V, ttl time.Duration) {
	c.store.Store(key, &cacheItem[K, V]{value, time.Now().Add(ttl)})

	if c.writes.Add(1)%sweepEvery == 0 {
		c.sweepExpired()
	}
}

// sweepExpired removes every entry whose TTL has elapsed.
func (c *Cache[K, V]) sweepExpired() {
	if !c.sweeping.CompareAndSwap(false, true) {
		return
	}
	defer c.sweeping.Store(false)

	now := time.Now()
	c.store.Range(func(key, value interface{}) bool {
		if ci, ok := value.(*cacheItem[K, V]); ok && now.After(ci.expiration) {
			c.store.Delete(key)
		}
		return true
	})
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	item, ok := c.store.Load(key)
	if !ok {
		return (&cacheItem[K, V]{}).value, false
	}
	ci := item.(*cacheItem[K, V])
	if time.Now().After(ci.expiration) {
		c.Delete(key)
		return (&cacheItem[K, V]{}).value, false
	}
	return ci.value, true
}

func (c *Cache[K, V]) Pop() (K, V, bool) {
	var key K
	var value V
	var ok bool
	c.store.Range(func(k, v interface{}) bool {
		key = k.(K)
		value = v.(*cacheItem[K, V]).value
		ok = true
		c.store.Delete(k)
		return false
	})
	return key, value, ok
}

func (c *Cache[K, V]) Has(key K) bool {
	_, ok := c.store.Load(key)
	return ok
}

func (c *Cache[K, V]) GetOrSet(key K, createFunc func() (V, error)) (V, error) {
	value, ok := c.Get(key)
	if ok {
		return value, nil
	}

	newValue, err := createFunc()
	if err != nil {
		return newValue, err
	}
	c.Set(key, newValue)
	return newValue, nil
}

func (c *Cache[K, V]) Delete(key K) {
	c.store.Delete(key)
}

func (c *Cache[K, V]) Clear() {
	c.store.Range(func(key interface{}, value interface{}) bool {
		c.store.Delete(key)
		return true
	})
}

func (c *Cache[K, V]) Range(callback func(key K, value V) bool) {
	c.store.Range(func(key, value interface{}) bool {
		ci := value.(*cacheItem[K, V])
		return callback(key.(K), ci.value)
	})
}
