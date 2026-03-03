package repository

import (
	"container/list"
	"sync"
	"time"
)

// localCache is a small LRU cache for hot reads.
type localCache[T any] struct {
	mu       sync.Mutex
	maxSize  int
	ttl      time.Duration
	items    map[string]*list.Element
	lru      *list.List
	cleaner  *time.Ticker
	stopChan chan struct{}
}

type cacheEntry[T any] struct {
	key      string
	value    T
	expireAt time.Time
}

func newLocalCache[T any](maxSize int, ttl time.Duration) *localCache[T] {
	if maxSize <= 0 || ttl <= 0 {
		return nil
	}
	c := &localCache[T]{
		maxSize:  maxSize,
		ttl:      ttl,
		items:    make(map[string]*list.Element, maxSize),
		lru:      list.New(),
		cleaner:  time.NewTicker(ttl),
		stopChan: make(chan struct{}),
	}
	go c.cleanupLoop()
	return c
}

func (c *localCache[T]) Get(key string) (T, bool) {
	var zero T
	if c == nil {
		return zero, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	elem, ok := c.items[key]
	if !ok {
		return zero, false
	}
	entry := elem.Value.(*cacheEntry[T])
	if time.Now().After(entry.expireAt) {
		c.removeElement(elem)
		return zero, false
	}
	c.lru.MoveToFront(elem)
	return entry.value, true
}

func (c *localCache[T]) Set(key string, value T) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		entry := elem.Value.(*cacheEntry[T])
		entry.value = value
		entry.expireAt = time.Now().Add(c.ttl)
		c.lru.MoveToFront(elem)
		return
	}
	entry := &cacheEntry[T]{
		key:      key,
		value:    value,
		expireAt: time.Now().Add(c.ttl),
	}
	elem := c.lru.PushFront(entry)
	c.items[key] = elem
	if c.lru.Len() > c.maxSize {
		c.evictOldest()
	}
}

func (c *localCache[T]) Delete(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

func (c *localCache[T]) evictOldest() {
	if c.lru.Len() == 0 {
		return
	}
	elem := c.lru.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

func (c *localCache[T]) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry[T])
	delete(c.items, entry.key)
	c.lru.Remove(elem)
}

func (c *localCache[T]) cleanupLoop() {
	for {
		select {
		case <-c.cleaner.C:
			c.cleanup()
		case <-c.stopChan:
			return
		}
	}
}

func (c *localCache[T]) cleanup() {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for elem := c.lru.Back(); elem != nil; {
		prev := elem.Prev()
		entry := elem.Value.(*cacheEntry[T])
		if now.After(entry.expireAt) {
			c.removeElement(elem)
		}
		elem = prev
	}
}
