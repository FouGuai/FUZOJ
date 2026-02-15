package db

import (
	"fmt"
	"sync/atomic"
)

// Provider returns the current database instance.
type Provider interface {
	Current() Database
}

// StaticProvider always returns the same database instance.
type StaticProvider struct {
	db Database
}

// NewStaticProvider creates a new StaticProvider.
func NewStaticProvider(database Database) *StaticProvider {
	return &StaticProvider{db: database}
}

// Current returns the configured database instance.
func (p *StaticProvider) Current() Database {
	if p == nil {
		return nil
	}
	return p.db
}

// Manager supports swapping the current database instance atomically.
type Manager struct {
	current atomic.Value
}

// NewManager creates a new Manager with the provided database instance.
func NewManager(database Database) *Manager {
	m := &Manager{}
	m.current.Store(database)
	return m
}

// Current returns the active database instance.
func (m *Manager) Current() Database {
	if m == nil {
		return nil
	}
	value := m.current.Load()
	if value == nil {
		return nil
	}
	return value.(Database)
}

// Swap replaces the current database instance and returns the previous one.
func (m *Manager) Swap(next Database) Database {
	prev := m.Current()
	m.current.Store(next)
	return prev
}

// CurrentDatabase fetches the current database instance from provider.
func CurrentDatabase(provider Provider) (Database, error) {
	if provider == nil {
		return nil, fmt.Errorf("database provider is nil")
	}
	database := provider.Current()
	if database == nil {
		return nil, fmt.Errorf("database is nil")
	}
	return database, nil
}
