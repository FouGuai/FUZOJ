package repository

import "time"

const (
	ProblemStatusDraft     int32 = 0
	ProblemStatusPublished int32 = 1
	ProblemStatusArchived  int32 = 2
)

// Problem represents the basic problem entity.
type Problem struct {
	ID        int64
	Title     string
	Status    int32
	OwnerID   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
