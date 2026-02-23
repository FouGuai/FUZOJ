package middleware

import "time"

// RateLimitPolicy defines per-route rate limit overrides.
type RateLimitPolicy struct {
	Window   time.Duration
	UserMax  int
	IPMax    int
	RouteMax int
}

// RoutePolicy holds gateway route policies.
type RoutePolicy struct {
	Name        string
	Path        string
	Auth        AuthPolicy
	RateLimit   RateLimitPolicy
	Timeout     time.Duration
	StripPrefix string
}
