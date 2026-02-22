package engine

import (
	"fuzoj/services/judge_service/internal/sandbox/security"
	"fuzoj/services/judge_service/internal/sandbox/spec"
)

type initRequest struct {
	RunSpec       spec.RunSpec
	Isolation     security.IsolationProfile
	EnableSeccomp bool
	EnableNs      bool
}
