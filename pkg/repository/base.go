package repository

import "context"

// Repository defines the generic base interface for data access operations.
// This interface uses Go generics (1.18+) to provide type-safe CRUD operations
// for any entity type without code duplication.
//
// T is the entity type (e.g., User, Problem, Submission)
type Repository[T any] interface {
	// Create inserts a new entity into the database
	// Returns an error if the operation fails
	Create(ctx context.Context, entity *T) error

	// GetByID retrieves an entity by its primary key
	// Returns nil and ErrNotFound if the entity doesn't exist
	GetByID(ctx context.Context, id int64) (*T, error)

	// Update modifies an existing entity in the database
	// Returns an error if the entity doesn't exist or the operation fails
	Update(ctx context.Context, entity *T) error

	// Delete removes an entity by its primary key
	// Returns an error if the entity doesn't exist or the operation fails
	Delete(ctx context.Context, id int64) error

	// List retrieves a list of entities with optional filtering, sorting, and pagination
	// Returns the entities and the total count (for pagination)
	List(ctx context.Context, opts ListOptions) ([]*T, int64, error)

	// BatchCreate inserts multiple entities in a single transaction
	// More efficient than calling Create multiple times
	BatchCreate(ctx context.Context, entities []*T) error

	// BatchDelete removes multiple entities by their IDs
	BatchDelete(ctx context.Context, ids []int64) error

	// Exists checks if an entity with the given ID exists
	// Returns true if exists, false otherwise
	Exists(ctx context.Context, id int64) (bool, error)

	// Count returns the total number of entities matching the given filters
	Count(ctx context.Context, filters map[string]interface{}) (int64, error)
}
