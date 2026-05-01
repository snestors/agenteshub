package realtime

import (
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second

type cachedEntry struct {
	value     *RealtimeUsage
	expiresAt time.Time
}

// Cache is a simple in-memory TTL cache keyed by provider name.
// On fetch error it returns the previous valid value alongside the error.
type Cache struct {
	mu      sync.Mutex
	entries map[string]cachedEntry
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{entries: make(map[string]cachedEntry)}
}

// Get returns the cached entry for key and whether it is still valid.
func (c *Cache) Get(key string) (*RealtimeUsage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	return e.value, time.Now().Before(e.expiresAt)
}

// Set stores value under key with the standard TTL.
func (c *Cache) Set(key string, value *RealtimeUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedEntry{
		value:     value,
		expiresAt: time.Now().Add(cacheTTL),
	}
}
