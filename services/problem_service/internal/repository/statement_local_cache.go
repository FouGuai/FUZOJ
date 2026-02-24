package repository

import (
	"container/list"
	"sync"
	"time"
)

// StatementLocalCache is a small LRU cache for hot statement reads.
type StatementLocalCache struct {
	mu      sync.Mutex
	maxSize int
	ttl     time.Duration
	ll      *list.List
	cache   map[string]*list.Element
}

type statementCacheEntry struct {
	key       string
	value     ProblemStatement
	expiresAt time.Time
}

func NewStatementLocalCache(maxSize int, ttl time.Duration) *StatementLocalCache {
	if maxSize <= 0 {
		maxSize = 1
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &StatementLocalCache{
		maxSize: maxSize,
		ttl:     ttl,
		ll:      list.New(),
		cache:   make(map[string]*list.Element, maxSize),
	}
}

func (c *StatementLocalCache) Get(key string) (ProblemStatement, bool) {
	if c == nil {
		return ProblemStatement{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		entry := elem.Value.(statementCacheEntry)
		if time.Now().After(entry.expiresAt) {
			c.removeElement(elem)
			return ProblemStatement{}, false
		}
		c.ll.MoveToFront(elem)
		return entry.value, true
	}
	return ProblemStatement{}, false
}

func (c *StatementLocalCache) Set(key string, value ProblemStatement, ttl time.Duration) {
	if c == nil {
		return
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		elem.Value = statementCacheEntry{
			key:       key,
			value:     value,
			expiresAt: time.Now().Add(ttl),
		}
		return
	}
	elem := c.ll.PushFront(statementCacheEntry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
	c.cache[key] = elem
	if c.ll.Len() > c.maxSize {
		c.evictOldest()
	}
}

func (c *StatementLocalCache) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
	}
}

func (c *StatementLocalCache) evictOldest() {
	if c == nil {
		return
	}
	elem := c.ll.Back()
	if elem == nil {
		return
	}
	c.removeElement(elem)
}

func (c *StatementLocalCache) removeElement(elem *list.Element) {
	if c == nil || elem == nil {
		return
	}
	c.ll.Remove(elem)
	entry := elem.Value.(statementCacheEntry)
	delete(c.cache, entry.key)
}
