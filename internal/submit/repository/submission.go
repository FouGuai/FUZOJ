package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"
)

const (
	defaultSubmissionCacheTTL      = 30 * time.Minute
	defaultSubmissionCacheEmptyTTL = 5 * time.Minute
	submissionCacheKeyPrefix       = "submission:"
)

var (
	ErrSubmissionNotFound = errors.New("submission not found")
)

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
	Create(ctx context.Context, tx db.Transaction, submission *Submission) error
	GetByID(ctx context.Context, tx db.Transaction, submissionID string) (*Submission, error)
}

// MySQLSubmissionRepository implements SubmissionRepository with MySQL.
type MySQLSubmissionRepository struct {
	db       db.Database
	cache    cache.Cache
	ttl      time.Duration
	emptyTTL time.Duration
}

// NewSubmissionRepository creates a submission repository with defaults.
func NewSubmissionRepository(database db.Database, cacheClient cache.Cache) SubmissionRepository {
	return NewSubmissionRepositoryWithTTL(database, cacheClient, defaultSubmissionCacheTTL, defaultSubmissionCacheEmptyTTL)
}

// NewSubmissionRepositoryWithTTL creates a submission repository with custom TTL.
func NewSubmissionRepositoryWithTTL(database db.Database, cacheClient cache.Cache, ttl, emptyTTL time.Duration) SubmissionRepository {
	if ttl <= 0 {
		ttl = defaultSubmissionCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultSubmissionCacheEmptyTTL
	}
	return &MySQLSubmissionRepository{
		db:       database,
		cache:    cacheClient,
		ttl:      ttl,
		emptyTTL: emptyTTL,
	}
}

const submissionColumns = "submission_id, problem_id, user_id, contest_id, language_id, source_code, source_key, source_hash, scene, created_at"

// Create inserts a submission record.
func (r *MySQLSubmissionRepository) Create(ctx context.Context, tx db.Transaction, submission *Submission) error {
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

	query := `
		INSERT INTO submissions
		(submission_id, problem_id, user_id, contest_id, language_id, source_code, source_key, source_hash, scene)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.GetQuerier(r.db, tx).Exec(
		ctx,
		query,
		submission.SubmissionID,
		submission.ProblemID,
		submission.UserID,
		submission.ContestID,
		submission.LanguageID,
		submission.SourceCode,
		submission.SourceKey,
		submission.SourceHash,
		submission.Scene,
	)
	if err != nil {
		return err
	}
	if r.cache != nil && tx == nil {
		r.setCache(ctx, submission)
	}
	return nil
}

// GetByID retrieves a submission by id.
func (r *MySQLSubmissionRepository) GetByID(ctx context.Context, tx db.Transaction, submissionID string) (*Submission, error) {
	if submissionID == "" {
		return nil, errors.New("submissionID is required")
	}
	if r.cache != nil && tx == nil {
		submission, err := cache.GetWithCached[*Submission](
			ctx,
			r.cache,
			submissionCacheKey(submissionID),
			cache.JitterTTL(r.ttl),
			cache.JitterTTL(r.emptyTTL),
			func(submission *Submission) bool { return submission == nil },
			marshalSubmission,
			unmarshalSubmission,
			func(ctx context.Context) (*Submission, error) {
				submission, err := r.getByIDFromDB(ctx, nil, submissionID)
				if err != nil {
					if errors.Is(err, ErrSubmissionNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return submission, nil
			},
		)
		if err != nil {
			return nil, err
		}
		if submission == nil {
			return nil, ErrSubmissionNotFound
		}
		return submission, nil
	}
	return r.getByIDFromDB(ctx, tx, submissionID)
}

func (r *MySQLSubmissionRepository) getByIDFromDB(ctx context.Context, tx db.Transaction, submissionID string) (*Submission, error) {
	query := "SELECT " + submissionColumns + " FROM submissions WHERE submission_id = ? LIMIT 1"
	row := db.GetQuerier(r.db, tx).QueryRow(ctx, query, submissionID)
	submission := &Submission{}
	var contestID *string
	if err := row.Scan(
		&submission.SubmissionID,
		&submission.ProblemID,
		&submission.UserID,
		&contestID,
		&submission.LanguageID,
		&submission.SourceCode,
		&submission.SourceKey,
		&submission.SourceHash,
		&submission.Scene,
		&submission.CreatedAt,
	); err != nil {
		if db.IsNoRows(err) {
			return nil, ErrSubmissionNotFound
		}
		return nil, err
	}
	if contestID != nil {
		submission.ContestID = *contestID
	}
	return submission, nil
}

func (r *MySQLSubmissionRepository) setCache(ctx context.Context, submission *Submission) {
	if submission == nil || r.cache == nil {
		return
	}
	payload := marshalSubmission(submission)
	if payload == "" {
		return
	}
	_ = r.cache.Set(ctx, submissionCacheKey(submission.SubmissionID), payload, cache.JitterTTL(r.ttl))
}

func submissionCacheKey(submissionID string) string {
	return submissionCacheKeyPrefix + submissionID
}

func marshalSubmission(submission *Submission) string {
	if submission == nil {
		return ""
	}
	data, err := json.Marshal(submission)
	if err != nil {
		return ""
	}
	return string(data)
}

func unmarshalSubmission(data string) (*Submission, error) {
	if data == "" || data == cache.NullCacheValue {
		return nil, nil
	}
	var submission Submission
	if err := json.Unmarshal([]byte(data), &submission); err != nil {
		return nil, err
	}
	return &submission, nil
}
