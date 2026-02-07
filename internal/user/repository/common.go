package repository

import (
	"context"
	"database/sql"
	"errors"

	"fuzoj/internal/common/db"

	"github.com/lib/pq"
)

const (
	userBannedKey     = "user:banned"
	tokenBlacklistKey = "token:blacklist"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrToeknNotFound  = errors.New("token not found")
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

func uniqueViolation(err error) (*pq.Error, bool) {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && string(pqErr.Code) == "23505" {
		return pqErr, true
	}
	return nil, false
}
