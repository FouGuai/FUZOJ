package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"fuzoj/internal/common/db"

	"github.com/go-sql-driver/mysql"
)

const (
	userBannedKey     = "user:banned"
	tokenBlacklistKey = "token:blacklist"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrTokenNotFound  = errors.New("token not found")
	ErrDuplicate      = errors.New("record already exists")
	ErrUsernameExists = errors.New("username already exists")
	ErrEmailExists    = errors.New("email already exists")
)

type dbQuerier interface {
	Query(ctx context.Context, query string, args ...interface{}) (db.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) db.Row
	Exec(ctx context.Context, query string, args ...interface{}) (db.Result, error)
}

func getQuerier(database db.Database, tx db.Transaction) dbQuerier {
	if tx != nil {
		return tx
	}
	return database
}

func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func uniqueViolation(err error) (string, bool) {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == 1062 {
		return extractDuplicateKeyName(myErr.Message), true
	}
	return "", false
}

func extractDuplicateKeyName(message string) string {
	if message == "" {
		return ""
	}
	const marker = "for key "
	idx := strings.LastIndex(message, marker)
	if idx == -1 {
		return ""
	}
	key := strings.TrimSpace(message[idx+len(marker):])
	return strings.Trim(key, " `\"'")
}
