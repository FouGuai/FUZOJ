package middleware

import "strings"

type wildcardPolicy struct {
	method string
	prefix string
	policy RoutePolicy
}

// PolicyMatcher matches requests to route policies.
type PolicyMatcher struct {
	exact    map[string]RoutePolicy
	wildcard []wildcardPolicy
}

func NewPolicyMatcher() *PolicyMatcher {
	return &PolicyMatcher{exact: make(map[string]RoutePolicy)}
}

func (m *PolicyMatcher) AddExact(method, path string, policy RoutePolicy) {
	if method == "" || path == "" {
		return
	}
	m.exact[keyFor(method, path)] = policy
}

func (m *PolicyMatcher) AddWildcard(method, prefix string, policy RoutePolicy) {
	if method == "" || prefix == "" {
		return
	}
	m.wildcard = append(m.wildcard, wildcardPolicy{method: strings.ToUpper(method), prefix: prefix, policy: policy})
}

func (m *PolicyMatcher) Match(method, path string) (RoutePolicy, bool) {
	if m == nil {
		return RoutePolicy{}, false
	}
	key := keyFor(method, path)
	if policy, ok := m.exact[key]; ok {
		return policy, true
	}

	var best RoutePolicy
	bestLen := -1
	method = strings.ToUpper(method)
	for _, item := range m.wildcard {
		if item.method != method {
			continue
		}
		if strings.HasPrefix(path, item.prefix) {
			if len(item.prefix) > bestLen {
				best = item.policy
				bestLen = len(item.prefix)
			}
		}
	}
	if bestLen >= 0 {
		return best, true
	}
	return RoutePolicy{}, false
}

func keyFor(method, path string) string {
	return strings.ToUpper(method) + " " + path
}
