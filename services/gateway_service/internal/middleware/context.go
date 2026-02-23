package middleware

import (
	"context"
	"net/http"

	"fuzoj/pkg/utils/contextkey"
)

type ctxKey string

const (
	ctxKeyUserRole  ctxKey = "user_role"
	ctxKeyRouteName ctxKey = "route_name"
	ctxKeyRoutePath ctxKey = "route_path"
	ctxKeyPolicy    ctxKey = "route_policy"
)

func withUserInfo(ctx context.Context, userID int64, role string) context.Context {
	ctx = context.WithValue(ctx, contextkey.UserID, userID)
	return context.WithValue(ctx, ctxKeyUserRole, role)
}

func withRoutePolicy(ctx context.Context, policy RoutePolicy) context.Context {
	return context.WithValue(ctx, ctxKeyPolicy, policy)
}

func getRoutePolicy(ctx context.Context) RoutePolicy {
	policy, ok := ctx.Value(ctxKeyPolicy).(RoutePolicy)
	if !ok {
		return RoutePolicy{}
	}
	return policy
}

func getUserRole(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(ctxKeyUserRole).(string)
	return role, ok
}

func withRouteName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, ctxKeyRouteName, name)
}

func getRouteName(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(ctxKeyRouteName).(string)
	return name, ok
}

func withRoutePath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ctxKeyRoutePath, path)
}

func getRoutePath(ctx context.Context) (string, bool) {
	path, ok := ctx.Value(ctxKeyRoutePath).(string)
	return path, ok
}

func getUserID(r *http.Request) (int64, bool) {
	val := r.Context().Value(contextkey.UserID)
	switch v := val.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case string:
		return 0, false
	default:
		return 0, false
	}
}
