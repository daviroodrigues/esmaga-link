package service

import (
	"sync"
	"time"
)

type l1Entry struct {
	value     string
	expiresAt int64
}

type l1Cache struct {
	mu         sync.RWMutex
	items      map[string]l1Entry
	maxEntries int
}

func newL1Cache(maxEntries int) *l1Cache {
	if maxEntries <= 0 {
		maxEntries = 50000
	}
	return &l1Cache{
		items:      make(map[string]l1Entry),
		maxEntries: maxEntries,
	}
}

func (c *l1Cache) Get(key string) (string, bool) {
	now := time.Now().UnixNano()
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || now >= entry.expiresAt {
		return "", false
	}
	return entry.value, true
}

func (c *l1Cache) Set(key, value string, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	now := time.Now().UnixNano()
	exp := now + ttl.Nanoseconds()

	c.mu.Lock()
	if len(c.items) >= c.maxEntries {

		for k, v := range c.items {
			if now >= v.expiresAt {
				delete(c.items, k)
			}
		}
		if len(c.items) >= c.maxEntries {
			for k := range c.items {
				delete(c.items, k)
				break
			}
		}
	}
	c.items[key] = l1Entry{value: value, expiresAt: exp}
	c.mu.Unlock()
}

func (c *l1Cache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}
