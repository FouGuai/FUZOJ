package repository

import (
	"context"
	"database/sql"
)

// sqlRunner defines the minimal DB operations for repositories.
type sqlRunner interface {
	ExecCtx(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowCtx(ctx context.Context, v any, query string, args ...any) error
	QueryRowsCtx(ctx context.Context, v any, query string, args ...any) error
}
