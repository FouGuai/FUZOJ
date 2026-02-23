package middleware

import "net/http"

// RoutePolicyMiddleware resolves and stores route policy in request context.
func RoutePolicyMiddleware(matcher *PolicyMatcher) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if matcher != nil {
				if policy, ok := matcher.Match(r.Method, r.URL.Path); ok {
					ctx := withRoutePolicy(r.Context(), policy)
					if policy.Name != "" {
						ctx = withRouteName(ctx, policy.Name)
					}
					if policy.Path != "" {
						ctx = withRoutePath(ctx, policy.Path)
					}
					r = r.WithContext(ctx)
				}
			}
			next(w, r)
		}
	}
}
