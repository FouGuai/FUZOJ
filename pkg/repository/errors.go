package repository

import "errors"

var (
	// ErrNotFound is returned when a requested entity is not found
	ErrNotFound = errors.New("entity not found")

	// ErrAlreadyExists is returned when attempting to create an entity that already exists
	ErrAlreadyExists = errors.New("entity already exists")

	// ErrInvalidID is returned when an invalid ID is provided
	ErrInvalidID = errors.New("invalid entity ID")

	// ErrInvalidInput is returned when input validation fails
	ErrInvalidInput = errors.New("invalid input")

	// ErrConflict is returned when there's a conflict (e.g., optimistic locking)
	ErrConflict = errors.New("entity conflict detected")

	// ErrConnectionFailed is returned when database connection fails
	ErrConnectionFailed = errors.New("database connection failed")

	// ErrQueryFailed is returned when a query execution fails
	ErrQueryFailed = errors.New("query execution failed")

	// ErrTransactionFailed is returned when a transaction fails
	ErrTransactionFailed = errors.New("transaction failed")

	// ErrDeadlock is returned when a database deadlock is detected
	ErrDeadlock = errors.New("database deadlock detected")

	// ErrTimeout is returned when a query times out
	ErrTimeout = errors.New("query timeout")

	// ErrNoRowsAffected is returned when an update/delete affects no rows
	ErrNoRowsAffected = errors.New("no rows affected")
)

// IsNotFoundError checks if an error is a "not found" error
func IsNotFoundError(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflictError checks if an error is a conflict error
func IsConflictError(err error) bool {
	return errors.Is(err, ErrConflict) || errors.Is(err, ErrAlreadyExists)
}

// IsConnectionError checks if an error is a connection error
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}
