package discovery

import (
	"fmt"
	"sort"
	"sync/atomic"
)

// RoundRobinPicker selects targets in round-robin order.
type RoundRobinPicker struct {
	targets atomic.Value
	cursor  uint64
}

// NewRoundRobinPicker creates a picker with initial targets.
func NewRoundRobinPicker(targets []string) *RoundRobinPicker {
	picker := &RoundRobinPicker{}
	picker.UpdateTargets(targets)
	return picker
}

// UpdateTargets replaces targets with a sorted snapshot.
func (p *RoundRobinPicker) UpdateTargets(targets []string) {
	snapshot := append([]string(nil), targets...)
	sort.Strings(snapshot)
	p.targets.Store(snapshot)
}

// Pick returns one target in round-robin order.
func (p *RoundRobinPicker) Pick() (string, error) {
	val := p.targets.Load()
	if val == nil {
		return "", fmt.Errorf("no available targets")
	}
	targets := val.([]string)
	if len(targets) == 0 {
		return "", fmt.Errorf("no available targets")
	}
	index := atomic.AddUint64(&p.cursor, 1)
	return targets[int(index-1)%len(targets)], nil
}
