package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fuzoj/services/submit_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrSubmissionNotFound = errors.New("submission not found")

// Submission represents a judge submission record.
type Submission struct {
	SubmissionID string
	ProblemID    int64
	UserID       int64
	ContestID    string
	LanguageID   string
	SourceCode   string
	SourceKey    string
	SourceHash   string
	Scene        string
	CreatedAt    time.Time
}

// SubmissionRepository defines submission persistence interfaces.
type SubmissionRepository interface {
	Create(ctx context.Context, session sqlx.Session, submission *Submission) error
	GetByID(ctx context.Context, session sqlx.Session, submissionID string) (*Submission, error)
}

// MySQLSubmissionRepository implements SubmissionRepository with MySQL.
type MySQLSubmissionRepository struct {
	model model.SubmissionsModel
}

// NewSubmissionRepository creates a submission repository.
func NewSubmissionRepository(submissionsModel model.SubmissionsModel) SubmissionRepository {
	return &MySQLSubmissionRepository{model: submissionsModel}
}

// Create inserts a submission record.
func (r *MySQLSubmissionRepository) Create(ctx context.Context, session sqlx.Session, submission *Submission) error {
	if submission == nil {
		return errors.New("submission is nil")
	}
	if submission.SubmissionID == "" {
		return errors.New("submissionID is required")
	}
	if submission.ProblemID <= 0 {
		return errors.New("problemID is required")
	}
	if submission.UserID <= 0 {
		return errors.New("userID is required")
	}
	if submission.LanguageID == "" {
		return errors.New("languageID is required")
	}
	if submission.SourceKey == "" {
		return errors.New("sourceKey is required")
	}
	if submission.SourceHash == "" {
		return errors.New("sourceHash is required")
	}

	data := &model.Submissions{
		SubmissionId: submission.SubmissionID,
		ProblemId:    submission.ProblemID,
		UserId:       submission.UserID,
		LanguageId:   submission.LanguageID,
		SourceCode:   submission.SourceCode,
		SourceKey:    submission.SourceKey,
		SourceHash:   submission.SourceHash,
		Scene:        submission.Scene,
	}
	if submission.ContestID != "" {
		data.ContestId = sql.NullString{String: submission.ContestID, Valid: true}
	}

	_, err := r.model.WithSession(session).Insert(ctx, data)
	return err
}

// GetByID retrieves a submission by id.
func (r *MySQLSubmissionRepository) GetByID(ctx context.Context, session sqlx.Session, submissionID string) (*Submission, error) {
	if submissionID == "" {
		return nil, errors.New("submissionID is required")
	}
	row, err := r.model.WithSession(session).FindOne(ctx, submissionID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	submission := &Submission{
		SubmissionID: row.SubmissionId,
		ProblemID:    row.ProblemId,
		UserID:       row.UserId,
		LanguageID:   row.LanguageId,
		SourceCode:   row.SourceCode,
		SourceKey:    row.SourceKey,
		SourceHash:   row.SourceHash,
		Scene:        row.Scene,
		CreatedAt:    row.CreatedAt,
	}
	if row.ContestId.Valid {
		submission.ContestID = row.ContestId.String
	}
	return submission, nil
}
