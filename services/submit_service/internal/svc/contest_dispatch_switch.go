package svc

import (
	"strings"
	"sync/atomic"
)

const (
	ContestDispatchModeRPC   = "rpc"
	ContestDispatchModeKafka = "kafka"
)

// ContestDispatchSwitchConfig defines runtime switch config.
type ContestDispatchSwitchConfig struct {
	Mode string `json:"mode"`
}

// ContestDispatchSwitch holds the runtime mode for contest dispatch.
type ContestDispatchSwitch struct {
	mode atomic.Value
}

func NewContestDispatchSwitch(initial string) *ContestDispatchSwitch {
	sw := &ContestDispatchSwitch{}
	sw.mode.Store(NormalizeContestDispatchMode(initial))
	return sw
}

func (s *ContestDispatchSwitch) Mode() string {
	if s == nil {
		return ContestDispatchModeRPC
	}
	if val, ok := s.mode.Load().(string); ok && val != "" {
		return val
	}
	return ContestDispatchModeRPC
}

func (s *ContestDispatchSwitch) Update(mode string) {
	if s == nil {
		return
	}
	s.mode.Store(NormalizeContestDispatchMode(mode))
}

// NormalizeContestDispatchMode normalizes switch mode.
func NormalizeContestDispatchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ContestDispatchModeKafka:
		return ContestDispatchModeKafka
	default:
		return ContestDispatchModeRPC
	}
}
