package user_service_test

import (
	"context"

	"fuzoj/pkg/utils/contextkey"
	"fuzoj/services/user_service/internal/svc"
)

func newTraceContext(traceID string) context.Context {
	return context.WithValue(context.Background(), contextkey.TraceID, traceID)
}

func newServiceContext(deps *testDeps) *svc.ServiceContext {
	if deps == nil {
		return &svc.ServiceContext{}
	}
	return deps.svcCtx
}
