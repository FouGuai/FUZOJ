package repository

import "time"

const (
	ProblemStatusDraft     int32 = 0
	ProblemStatusPublished int32 = 1
	ProblemStatusArchived  int32 = 2

	ProblemVersionStateDraft     int32 = 0
	ProblemVersionStatePublished int32 = 1
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

// ProblemLatestMeta represents latest published meta for a problem.
type ProblemLatestMeta struct {
	ProblemID    int64
	Version      int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
	UpdatedAt    time.Time
}

// ProblemStatement represents statement content for a problem version.
type ProblemStatement struct {
	ProblemID     int64
	Version       int32
	StatementMd   string
	StatementHash string
	UpdatedAt     time.Time
}

// UploadSession represents one multipart upload session.
type UploadSession struct {
	ID                int64
	ProblemID         int64
	Version           int32
	IdempotencyKey    string
	Bucket            string
	ObjectKey         string
	UploadID          string
	ExpectedSizeBytes int64
	ExpectedSHA256    string
	ContentType       string
	State             int32
	ExpiresAt         time.Time
	CreatedBy         int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ProblemVersionMeta represents version metadata for publish checks.
type ProblemVersionMeta struct {
	ProblemID    int64
	Version      int32
	State        int32
	ManifestHash string
	DataPackKey  string
	DataPackHash string
}

// CreateUploadSessionInput is the input for creating a new upload session.
type CreateUploadSessionInput struct {
	ProblemID      int64
	Version        int32
	IdempotencyKey string
	Bucket         string
	ObjectKey      string
	ExpiresAt      time.Time
	CreatedBy      int64

	ExpectedSizeBytes int64
	ExpectedSHA256    string
	ContentType       string
}
