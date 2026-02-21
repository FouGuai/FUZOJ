package engine

import "fuzoj/judge_service/internal/sandbox/security"

// ProfileResolver resolves a profile name into an isolation profile.
type ProfileResolver interface {
	Resolve(profile string) (security.IsolationProfile, error)
}

// Config controls sandbox engine behavior.
type Config struct {
	CgroupRoot           string
	SeccompDir           string
	HelperPath           string
	StdoutStderrMaxBytes int64
	EnableSeccomp        bool
	EnableCgroup         bool
	EnableNamespaces     bool
}
