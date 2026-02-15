package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/mq"
	"fuzoj/internal/common/storage"
	"fuzoj/internal/judge/model"
	judgeRepo "fuzoj/internal/judge/repository"
	"fuzoj/internal/judge/sandbox/result"
	"fuzoj/internal/submit/repository"
	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	idempotencyKeyPrefix = "submit:idempotency:"
	rateUserKeyPrefix    = "submit:rate:user:"
	rateIPKeyPrefix      = "submit:rate:ip:"
	defaultSourcePrefix  = "submissions"
	defaultBatchLimit    = 200
	processingMarker     = "processing"
)

// TopicConfig defines routing topics for judge tasks.
type TopicConfig struct {
	Level0 string
	Level1 string
	Level2 string
	Level3 string
}

// RateLimitConfig holds throttling configuration.
type RateLimitConfig struct {
	UserMax int
	IPMax   int
	Window  time.Duration
}

// TimeoutConfig holds timeout settings for external calls.
type TimeoutConfig struct {
	DB      time.Duration
	Cache   time.Duration
	MQ      time.Duration
	Storage time.Duration
	Status  time.Duration
}

// Config holds submit service dependencies and settings.
type Config struct {
	SubmissionRepo repository.SubmissionRepository
	StatusRepo     *judgeRepo.StatusRepository
	Storage        storage.ObjectStorage
	MQ             mq.MessageQueue
	Cache          cache.Cache

	Topics              TopicConfig
	FinalStatusHandlers []FinalStatusHandler
	SourceBucket        string
	SourceKeyPrefix     string
	MaxCodeBytes        int
	IdempotencyTTL      time.Duration
	BatchLimit          int
	RateLimit           RateLimitConfig
	Timeouts            TimeoutConfig
}

// SubmitService handles submission intake and dispatch.
type SubmitService struct {
	submissionRepo repository.SubmissionRepository
	statusRepo     *judgeRepo.StatusRepository
	storage        storage.ObjectStorage
	mq             mq.MessageQueue
	cache          cache.Cache

	topics              TopicConfig
	finalStatusHandlers []FinalStatusHandler
	sourceBucket        string
	sourceKeyPrefix     string
	maxCodeBytes        int
	idempotencyTTL      time.Duration
	batchLimit          int
	rateLimit           RateLimitConfig
	timeouts            TimeoutConfig
}

// SubmitInput describes a submission request.
type SubmitInput struct {
	ProblemID         int64
	UserID            int64
	LanguageID        string
	SourceCode        string
	ContestID         string
	Scene             string
	ExtraCompileFlags []string
	IdempotencyKey    string
	ClientIP          string
}

// NewSubmitService creates a new submit service.
func NewSubmitService(cfg Config) (*SubmitService, error) {
	if cfg.SubmissionRepo == nil {
		return nil, fmt.Errorf("submission repository is required")
	}
	if cfg.StatusRepo == nil {
		return nil, fmt.Errorf("status repository is required")
	}
	if cfg.Storage == nil {
		return nil, fmt.Errorf("storage is required")
	}
	if cfg.MQ == nil {
		return nil, fmt.Errorf("message queue is required")
	}
	if cfg.Cache == nil {
		return nil, fmt.Errorf("cache is required")
	}
	if cfg.SourceBucket == "" {
		return nil, fmt.Errorf("source bucket is required")
	}
	if cfg.SourceKeyPrefix == "" {
		cfg.SourceKeyPrefix = defaultSourcePrefix
	}
	if cfg.BatchLimit <= 0 {
		cfg.BatchLimit = defaultBatchLimit
	}
	return &SubmitService{
		submissionRepo:      cfg.SubmissionRepo,
		statusRepo:          cfg.StatusRepo,
		storage:             cfg.Storage,
		mq:                  cfg.MQ,
		cache:               cfg.Cache,
		topics:              cfg.Topics,
		finalStatusHandlers: cfg.FinalStatusHandlers,
		sourceBucket:        cfg.SourceBucket,
		sourceKeyPrefix:     cfg.SourceKeyPrefix,
		maxCodeBytes:        cfg.MaxCodeBytes,
		idempotencyTTL:      cfg.IdempotencyTTL,
		batchLimit:          cfg.BatchLimit,
		rateLimit:           cfg.RateLimit,
		timeouts:            cfg.Timeouts,
	}, nil
}

// Submit creates a submission and dispatches it to judge queues.
func (s *SubmitService) Submit(ctx context.Context, input SubmitInput) (string, model.JudgeStatusResponse, error) {
	if err := s.validateInput(input); err != nil {
		return "", model.JudgeStatusResponse{}, err
	}
	if err := s.checkRateLimit(ctx, input.UserID, input.ClientIP); err != nil {
		return "", model.JudgeStatusResponse{}, err
	}

	acquired, existingID, err := s.acquireIdempotency(ctx, input.IdempotencyKey)
	if err != nil {
		return "", model.JudgeStatusResponse{}, err
	}
	if !acquired && existingID != "" {
		status, statusErr := s.statusRepo.Get(ctx, existingID)
		if statusErr != nil {
			return "", model.JudgeStatusResponse{}, statusErr
		}
		return existingID, status, nil
	}

	submissionID := uuid.NewString()
	sourceHash := hashSource(input.SourceCode)
	sourceKey := s.buildSourceKey(submissionID)
	createdAt := time.Now()

	if err := s.uploadSource(ctx, sourceKey, input.SourceCode); err != nil {
		s.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", model.JudgeStatusResponse{}, err
	}

	submission := &repository.Submission{
		SubmissionID: submissionID,
		ProblemID:    input.ProblemID,
		UserID:       input.UserID,
		ContestID:    input.ContestID,
		LanguageID:   input.LanguageID,
		SourceCode:   input.SourceCode,
		SourceKey:    sourceKey,
		SourceHash:   sourceHash,
		Scene:        normalizeScene(input.Scene, input.ContestID),
		CreatedAt:    createdAt,
	}

	if err := s.createSubmission(ctx, submission); err != nil {
		s.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", model.JudgeStatusResponse{}, err
	}

	pending := model.JudgeStatusResponse{
		SubmissionID: submissionID,
		Status:       result.StatusPending,
		Timestamps:   result.Timestamps{ReceivedAt: createdAt.Unix()},
		Progress:     model.Progress{TotalTests: 0, DoneTests: 0},
	}
	if err := s.saveStatus(ctx, pending); err != nil {
		s.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", model.JudgeStatusResponse{}, err
	}

	if err := s.publishMessage(ctx, submission, input.ExtraCompileFlags); err != nil {
		s.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", model.JudgeStatusResponse{}, err
	}

	s.finalizeIdempotency(ctx, input.IdempotencyKey, submissionID, acquired)
	return submissionID, pending, nil
}

// GetStatus returns status for one submission.
func (s *SubmitService) GetStatus(ctx context.Context, submissionID string) (model.JudgeStatusResponse, error) {
	if submissionID == "" {
		return model.JudgeStatusResponse{}, appErr.ValidationError("submission_id", "required")
	}
	ctxStatus := withTimeout(ctx, s.timeouts.Status)
	defer ctxStatus.cancel()
	return s.statusRepo.Get(ctxStatus.ctx, submissionID)
}

// GetStatusBatch returns statuses for multiple submissions.
func (s *SubmitService) GetStatusBatch(ctx context.Context, submissionIDs []string) ([]model.JudgeStatusResponse, []string, error) {
	if len(submissionIDs) == 0 {
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	if len(submissionIDs) > s.batchLimit {
		return nil, nil, appErr.ValidationError("submission_ids", "too_many")
	}
	ctxStatus := withTimeout(ctx, s.timeouts.Status)
	defer ctxStatus.cancel()
	return s.statusRepo.GetBatch(ctxStatus.ctx, submissionIDs)
}

// GetSource returns stored source code for a submission.
func (s *SubmitService) GetSource(ctx context.Context, submissionID string) (*repository.Submission, error) {
	if submissionID == "" {
		return nil, appErr.ValidationError("submission_id", "required")
	}
	ctxDB := withTimeout(ctx, s.timeouts.DB)
	defer ctxDB.cancel()
	submission, err := s.submissionRepo.GetByID(ctxDB.ctx, nil, submissionID)
	if err != nil {
		if errors.Is(err, repository.ErrSubmissionNotFound) {
			return nil, appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
		}
		return nil, appErr.Wrapf(err, appErr.DatabaseError, "get submission failed")
	}
	return submission, nil
}

func (s *SubmitService) validateInput(input SubmitInput) error {
	if input.ProblemID <= 0 {
		return appErr.ValidationError("problem_id", "required")
	}
	if input.UserID <= 0 {
		return appErr.ValidationError("user_id", "required")
	}
	if strings.TrimSpace(input.LanguageID) == "" {
		return appErr.ValidationError("language_id", "required")
	}
	if strings.TrimSpace(input.SourceCode) == "" {
		return appErr.ValidationError("source_code", "required")
	}
	if s.maxCodeBytes > 0 && len([]byte(input.SourceCode)) > s.maxCodeBytes {
		return appErr.New(appErr.CodeTooLarge).WithMessage("source code too large")
	}
	return nil
}

func (s *SubmitService) acquireIdempotency(ctx context.Context, key string) (bool, string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return true, "", nil
	}
	cacheKey := idempotencyKeyPrefix + key
	ctxCache := withTimeout(ctx, s.timeouts.Cache)
	defer ctxCache.cancel()

	existing, err := s.cache.Get(ctxCache.ctx, cacheKey)
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "read idempotency key failed")
	}
	if existing != "" && existing != processingMarker {
		return false, existing, nil
	}

	ttl := s.idempotencyTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	ok, err := s.cache.SetNX(ctxCache.ctx, cacheKey, processingMarker, ttl)
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "reserve idempotency key failed")
	}
	if ok {
		return true, "", nil
	}
	existing, err = s.cache.Get(ctxCache.ctx, cacheKey)
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "read idempotency key failed")
	}
	if existing != "" && existing != processingMarker {
		return false, existing, nil
	}
	return false, "", appErr.New(appErr.TooManyRequests).WithMessage("request is processing")
}

func (s *SubmitService) finalizeIdempotency(ctx context.Context, key, submissionID string, acquired bool) {
	if !acquired || strings.TrimSpace(key) == "" {
		return
	}
	cacheKey := idempotencyKeyPrefix + strings.TrimSpace(key)
	ttl := s.idempotencyTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	ctxCache := withTimeout(ctx, s.timeouts.Cache)
	defer ctxCache.cancel()
	if err := s.cache.Set(ctxCache.ctx, cacheKey, submissionID, ttl); err != nil {
		logger.Warn(ctx, "update idempotency key failed", zap.Error(err))
	}
}

func (s *SubmitService) releaseIdempotency(ctx context.Context, key string, acquired bool) {
	if !acquired || strings.TrimSpace(key) == "" {
		return
	}
	cacheKey := idempotencyKeyPrefix + strings.TrimSpace(key)
	ctxCache := withTimeout(ctx, s.timeouts.Cache)
	defer ctxCache.cancel()
	if err := s.cache.Del(ctxCache.ctx, cacheKey); err != nil {
		logger.Warn(ctx, "release idempotency key failed", zap.Error(err))
	}
}

func (s *SubmitService) checkRateLimit(ctx context.Context, userID int64, clientIP string) error {
	if s.rateLimit.Window <= 0 || (s.rateLimit.UserMax <= 0 && s.rateLimit.IPMax <= 0) {
		return nil
	}
	ctxCache := withTimeout(ctx, s.timeouts.Cache)
	defer ctxCache.cancel()

	if s.rateLimit.UserMax > 0 && userID > 0 {
		if err := s.checkRateCounter(ctxCache.ctx, rateUserKeyPrefix+fmt.Sprintf("%d", userID), s.rateLimit.UserMax); err != nil {
			return err
		}
	}
	if s.rateLimit.IPMax > 0 && clientIP != "" {
		if err := s.checkRateCounter(ctxCache.ctx, rateIPKeyPrefix+clientIP, s.rateLimit.IPMax); err != nil {
			return err
		}
	}
	return nil
}

func (s *SubmitService) checkRateCounter(ctx context.Context, key string, max int) error {
	count, err := s.cache.Incr(ctx, key)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "rate limit check failed")
	}
	if count == 1 {
		_ = s.cache.Expire(ctx, key, s.rateLimit.Window)
	}
	if int(count) > max {
		return appErr.New(appErr.SubmitTooFrequently).WithMessage("submit too frequently")
	}
	return nil
}

func (s *SubmitService) uploadSource(ctx context.Context, objectKey, source string) error {
	sizeBytes := int64(len([]byte(source)))
	reader := io.NopCloser(strings.NewReader(source))
	defer reader.Close()
	ctxStorage := withTimeout(ctx, s.timeouts.Storage)
	defer ctxStorage.cancel()
	if err := s.storage.PutObject(ctxStorage.ctx, s.sourceBucket, objectKey, reader, sizeBytes, "text/plain; charset=utf-8"); err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "upload source failed")
	}
	return nil
}

func (s *SubmitService) createSubmission(ctx context.Context, submission *repository.Submission) error {
	ctxDB := withTimeout(ctx, s.timeouts.DB)
	defer ctxDB.cancel()
	if err := s.submissionRepo.Create(ctxDB.ctx, nil, submission); err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "create submission failed")
	}
	return nil
}

func (s *SubmitService) saveStatus(ctx context.Context, status model.JudgeStatusResponse) error {
	ctxStatus := withTimeout(ctx, s.timeouts.Status)
	defer ctxStatus.cancel()
	if err := s.statusRepo.Save(ctxStatus.ctx, status); err != nil {
		return err
	}
	return nil
}

func (s *SubmitService) publishMessage(ctx context.Context, submission *repository.Submission, extraFlags []string) error {
	payload := model.JudgeMessage{
		SubmissionID:      submission.SubmissionID,
		ProblemID:         submission.ProblemID,
		LanguageID:        submission.LanguageID,
		SourceKey:         submission.SourceKey,
		SourceHash:        submission.SourceHash,
		ContestID:         submission.ContestID,
		UserID:            fmt.Sprintf("%d", submission.UserID),
		Priority:          resolvePriority(submission.Scene),
		ExtraCompileFlags: extraFlags,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "encode judge message failed")
	}
	message := mq.NewMessage(body)
	message.ID = submission.SubmissionID
	message.Priority = uint8(resolvePriority(submission.Scene))

	topic := resolveTopic(submission.Scene, s.topics)
	if topic == "" {
		return appErr.New(appErr.SubmissionCreateFailed).WithMessage("judge topic is not configured")
	}
	ctxMQ := withTimeout(ctx, s.timeouts.MQ)
	defer ctxMQ.cancel()
	if err := s.mq.Publish(ctxMQ.ctx, topic, message); err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "publish judge message failed")
	}
	return nil
}

func (s *SubmitService) buildSourceKey(submissionID string) string {
	return fmt.Sprintf("%s/%s/source.code", s.sourceKeyPrefix, submissionID)
}

func hashSource(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

func normalizeScene(scene, contestID string) string {
	scene = strings.TrimSpace(strings.ToLower(scene))
	if scene == "" && contestID != "" {
		return "contest"
	}
	if scene == "" {
		return "practice"
	}
	return scene
}

func resolvePriority(scene string) int {
	switch strings.ToLower(scene) {
	case "contest":
		return 0
	case "practice":
		return 1
	case "custom":
		return 2
	case "rejudge":
		return 3
	default:
		return 1
	}
}

func resolveTopic(scene string, topics TopicConfig) string {
	switch strings.ToLower(scene) {
	case "contest":
		return topics.Level0
	case "practice":
		return topics.Level1
	case "custom":
		return topics.Level2
	case "rejudge":
		return topics.Level3
	default:
		return topics.Level1
	}
}

type timeoutCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func withTimeout(ctx context.Context, timeout time.Duration) timeoutCtx {
	if timeout <= 0 {
		return timeoutCtx{ctx: ctx, cancel: func() {}}
	}
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	return timeoutCtx{ctx: ctxTimeout, cancel: cancel}
}
