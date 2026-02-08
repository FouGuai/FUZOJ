package engine

import (
	"fuzoj/internal/judge/sandbox/security"
	"fuzoj/internal/judge/sandbox/spec"
)

type initRequest struct {
	RunSpec       spec.RunSpec
	Isolation     security.IsolationProfile
	EnableSeccomp bool
	EnableNs      bool
}
