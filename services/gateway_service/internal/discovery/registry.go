package discovery

import (
	"fmt"
	"sync"

	"github.com/zeromicro/go-zero/core/discov"
)

// Picker selects a target address.
type Picker interface {
	Pick() (string, error)
}

// RegistryPicker keeps targets from etcd and selects with round-robin.
type RegistryPicker struct {
	key    string
	sub    *discov.Subscriber
	picker *RoundRobinPicker
}

// NewRegistryPicker creates a picker that watches registry key.
func NewRegistryPicker(etcdConf discov.EtcdConf, key string) (*RegistryPicker, error) {
	if err := etcdConf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid etcd config: %w", err)
	}
	if key == "" {
		return nil, fmt.Errorf("registry key is required")
	}

	opts := make([]discov.SubOption, 0, 2)
	if etcdConf.HasAccount() {
		opts = append(opts, discov.WithSubEtcdAccount(etcdConf.User, etcdConf.Pass))
	}
	if etcdConf.HasTLS() {
		opts = append(opts, discov.WithSubEtcdTLS(etcdConf.CertFile, etcdConf.CertKeyFile, etcdConf.CACertFile, etcdConf.InsecureSkipVerify))
	}

	sub, err := discov.NewSubscriber(etcdConf.Hosts, key, opts...)
	if err != nil {
		return nil, fmt.Errorf("create registry subscriber failed: %w", err)
	}

	picker := NewRoundRobinPicker(sub.Values())
	sub.AddListener(func() {
		picker.UpdateTargets(sub.Values())
	})

	return &RegistryPicker{
		key:    key,
		sub:    sub,
		picker: picker,
	}, nil
}

// Pick returns a target from registry.
func (p *RegistryPicker) Pick() (string, error) {
	return p.picker.Pick()
}

// Close stops watching registry.
func (p *RegistryPicker) Close() {
	if p.sub != nil {
		p.sub.Close()
	}
}

// RegistryManager manages registry pickers per key.
type RegistryManager struct {
	etcdConf discov.EtcdConf
	lock     sync.Mutex
	pickers  map[string]*RegistryPicker
}

// NewRegistryManager creates a new manager.
func NewRegistryManager(etcdConf discov.EtcdConf) (*RegistryManager, error) {
	if err := etcdConf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid etcd config: %w", err)
	}
	return &RegistryManager{
		etcdConf: etcdConf,
		pickers:  make(map[string]*RegistryPicker),
	}, nil
}

// GetPicker returns a picker for the given key.
func (m *RegistryManager) GetPicker(key string) (Picker, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if picker, ok := m.pickers[key]; ok {
		return picker, nil
	}
	picker, err := NewRegistryPicker(m.etcdConf, key)
	if err != nil {
		return nil, err
	}
	m.pickers[key] = picker
	return picker, nil
}

// Close closes all pickers.
func (m *RegistryManager) Close() {
	m.lock.Lock()
	defer m.lock.Unlock()
	for _, picker := range m.pickers {
		picker.Close()
	}
}
