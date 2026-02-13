package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Querier abstracts database operations for both database and transaction.
type Querier interface {
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) Row
	Exec(ctx context.Context, query string, args ...interface{}) (Result, error)
}

// GetQuerier returns transaction if provided, otherwise uses the database.
func GetQuerier(database Database, tx Transaction) Querier {
	if tx != nil {
		return tx
	}
	return database
}

// IsNoRows checks if the error is sql.ErrNoRows.
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// UniqueViolation inspects a MySQL duplicate key error and returns the key name.
func UniqueViolation(err error) (string, bool) {
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) && myErr.Number == 1062 {
		return ExtractDuplicateKeyName(myErr.Message), true
	}
	return "", false
}

// ExtractDuplicateKeyName parses duplicate key name from MySQL error message.
func ExtractDuplicateKeyName(message string) string {
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
