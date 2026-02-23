package repository

import (
	"container/list"
	"sync"
	"time"
)

type cacheEntry struct {
	key       string
	value     bool
	expiresAt time.Time
}

// LRUCache is a simple LRU cache with TTL support for hot-path checks.
type LRUCache struct {
	mu      sync.Mutex
	items   map[string]*list.Element
	order   *list.List
	maxSize int
	ttl     time.Duration
}

func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	if maxSize <= 0 {
		maxSize = 1024
	}
	return &LRUCache{
		items:   make(map[string]*list.Element, maxSize),
		order:   list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *LRUCache) Get(key string) (bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
			c.removeElement(elem)
			return false, false
		}
		c.order.MoveToFront(elem)
		return entry.value, true
	}
	return false, false
}

func (c *LRUCache) Set(key string, value bool, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	exp := time.Time{}
	if ttl == 0 {
		ttl = c.ttl
	}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}

	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry)
		entry.value = value
		entry.expiresAt = exp
		c.order.MoveToFront(elem)
		return
	}

	entry := &cacheEntry{key: key, value: value, expiresAt: exp}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
	if len(c.items) > c.maxSize {
		c.evictOldest()
	}
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

func (c *LRUCache) evictOldest() {
	elem := c.order.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
}

func (c *LRUCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	delete(c.items, entry.key)
	c.order.Remove(elem)
}
