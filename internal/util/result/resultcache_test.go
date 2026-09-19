package result

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheSetTRefreshDoesNotInheritThePreviousExpiry(t *testing.T) {
	// Each Set used to spawn a goroutine that slept out the whole TTL and then deleted the key.
	// Re-setting a key left the old timer running, so the refreshed value was dropped when the
	// value it replaced would have expired.
	cache := NewCache[string, int]()

	cache.SetT("key", 1, 30*time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	cache.SetT("key", 2, time.Minute)

	time.Sleep(40 * time.Millisecond)

	value, ok := cache.Get("key")
	require.True(t, ok, "the refreshed value should outlive the expiry of the one it replaced")
	require.Equal(t, 2, value)
}

func TestCacheGetExpiresLazily(t *testing.T) {
	cache := NewCache[string, int]()
	cache.SetT("key", 1, 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	_, ok := cache.Get("key")
	require.False(t, ok)
	require.False(t, cache.Has("key"), "an expired entry should be dropped when it is read")
}

func TestCacheSweepsExpiredEntries(t *testing.T) {
	cache := NewCache[int, int]()

	// Entries nobody reads again are evicted by the periodic sweep rather than by a timer each.
	for i := range sweepEvery {
		cache.SetT(i, i, time.Nanosecond)
	}
	time.Sleep(time.Millisecond)
	cache.SetT(-1, -1, time.Minute)
	cache.sweepExpired()

	remaining := 0
	cache.Range(func(int, int) bool {
		remaining++
		return true
	})
	require.Equal(t, 1, remaining)
}

func TestCacheConcurrentSetAndGet(t *testing.T) {
	cache := NewCache[int, int]()

	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 200 {
				cache.Set(w*200+i, i)
				cache.Get(i)
				cache.Has(i)
			}
		}()
	}
	wg.Wait()
}
