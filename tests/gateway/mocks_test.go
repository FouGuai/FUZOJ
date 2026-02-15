package gateway_test

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type mockSetCache struct {
	mu   sync.Mutex
	sets map[string]map[string]struct{}
}

func newMockSetCache() *mockSetCache {
	return &mockSetCache{sets: make(map[string]map[string]struct{})}
}

func (m *mockSetCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	if set == nil {
		set = make(map[string]struct{})
		m.sets[key] = set
	}
	for _, member := range members {
		set[fmt.Sprint(member)] = struct{}{}
	}
	return nil
}

func (m *mockSetCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	if set == nil {
		return nil
	}
	for _, member := range members {
		delete(set, fmt.Sprint(member))
	}
	return nil
}

func (m *mockSetCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	if set == nil {
		return false, nil
	}
	_, ok := set[fmt.Sprint(member)]
	return ok, nil
}

func (m *mockSetCache) SCard(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	if set == nil {
		return 0, nil
	}
	return int64(len(set)), nil
}

type mockBasicCache struct {
	mu      sync.Mutex
	values  map[string]int64
	expires map[string]time.Time
}

func newMockBasicCache() *mockBasicCache {
	return &mockBasicCache{values: make(map[string]int64), expires: make(map[string]time.Time)}
}

func (m *mockBasicCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		delete(m.values, key)
		delete(m.expires, key)
		return "", nil
	}
	if val, ok := m.values[key]; ok {
		return fmt.Sprintf("%d", val), nil
	}
	return "", nil
}

func (m *mockBasicCache) MGet(ctx context.Context, keys ...string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	results := make([]string, 0, len(keys))
	for _, key := range keys {
		if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
			delete(m.values, key)
			delete(m.expires, key)
			results = append(results, "")
			continue
		}
		if val, ok := m.values[key]; ok {
			results = append(results, fmt.Sprintf("%d", val))
			continue
		}
		results = append(results, "")
	}
	return results, nil
}

func (m *mockBasicCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = toInt64(value)
	if ttl > 0 {
		m.expires[key] = time.Now().Add(ttl)
	} else {
		delete(m.expires, key)
	}
	return nil
}

func (m *mockBasicCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		delete(m.values, key)
		delete(m.expires, key)
	}
	if _, ok := m.values[key]; ok {
		return false, nil
	}
	m.values[key] = toInt64(value)
	if ttl > 0 {
		m.expires[key] = time.Now().Add(ttl)
	} else {
		delete(m.expires, key)
	}
	return true, nil
}

func (m *mockBasicCache) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := ""
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		delete(m.values, key)
		delete(m.expires, key)
	}
	if val, ok := m.values[key]; ok {
		prev = fmt.Sprintf("%d", val)
	}
	m.values[key] = toInt64(value)
	delete(m.expires, key)
	return prev, nil
}

func (m *mockBasicCache) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.values, key)
		delete(m.expires, key)
	}
	return nil
}

func (m *mockBasicCache) Exists(ctx context.Context, keys ...string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var count int64
	for _, key := range keys {
		if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
			delete(m.values, key)
			delete(m.expires, key)
			continue
		}
		if _, ok := m.values[key]; ok {
			count++
		}
	}
	return count, nil
}

func (m *mockBasicCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ttl <= 0 {
		delete(m.expires, key)
		return nil
	}
	m.expires[key] = time.Now().Add(ttl)
	return nil
}

func (m *mockBasicCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok {
		return time.Until(exp), nil
	}
	return -1, nil
}

func (m *mockBasicCache) Incr(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		m.values[key] = 0
		delete(m.expires, key)
	}
	m.values[key]++
	return m.values[key], nil
}

func (m *mockBasicCache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		m.values[key] = 0
		delete(m.expires, key)
	}
	m.values[key] += value
	return m.values[key], nil
}

func (m *mockBasicCache) Decr(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		m.values[key] = 0
		delete(m.expires, key)
	}
	m.values[key]--
	return m.values[key], nil
}

func (m *mockBasicCache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.expires[key]; ok && time.Now().After(exp) {
		m.values[key] = 0
		delete(m.expires, key)
	}
	m.values[key] -= value
	return m.values[key], nil
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	case uint64:
		return int64(v)
	case uint32:
		return int64(v)
	case uint:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	default:
		return 0
	}
}
