package logic

import (
	"context"
	"time"
)

type timeoutCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func withTimeout(ctx context.Context, timeout time.Duration) timeoutCtx {
	if timeout <= 0 {
		return timeoutCtx{ctx: ctx, cancel: func() {}}
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx{ctx: ctxTimeout, cancel: cancel}
}
