package logic

import (
	"context"
	"fmt"

	"fuzoj/pkg/utils/contextkey"
)

func getClientIP(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if ip := ctx.Value(contextkey.ClientIP); ip != nil {
		return fmt.Sprint(ip)
	}
	return ""
}
