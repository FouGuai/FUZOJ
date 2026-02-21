package user_service_test

import (
	"context"

	"fuzoj/pkg/utils/contextkey"
	"fuzoj/user_service/internal/service"
	"fuzoj/user_service/internal/svc"
)

func newTraceContext(traceID string) context.Context {
	return context.WithValue(context.Background(), contextkey.TraceID, traceID)
}

func newServiceContext(auth *service.AuthService) *svc.ServiceContext {
	return &svc.ServiceContext{
		AuthService: auth,
	}
}
