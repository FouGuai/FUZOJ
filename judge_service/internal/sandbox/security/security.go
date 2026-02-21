// Package security defines sandbox isolation and security profiles.
package security

// IsolationProfile describes namespace and seccomp settings.
type IsolationProfile struct {
	RootFS         string
	SeccompProfile string
	DisableNetwork bool
}
