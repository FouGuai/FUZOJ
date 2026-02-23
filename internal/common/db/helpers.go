package db

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

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
