package submit_app

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

	"fuzoj/internal/common/storage"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_rpc_service/contestrpc"
	"fuzoj/services/submit_service/internal/domain"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/svc"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	idempotencyKeyPrefix = "submit:idempotency:"
	rateUserKeyPrefix    = "submit:rate:user:"
	rateIPKeyPrefix      = "submit:rate:ip:"
	defaultSourcePrefix  = "submissions"
	defaultBatchLimit    = 200
	processingMarker     = "processing"
)

// RateLimitConfig holds throttling configuration.
type RateLimitConfig struct {
	UserMax int
	IPMax   int
	Window  time.Duration
}

// TimeoutConfig holds timeout settings for external calls.
type TimeoutConfig struct {
	DB         time.Duration
	Cache      time.Duration
	MQ         time.Duration
	Storage    time.Duration
	Status     time.Duration
	ContestRPC time.Duration
}

type SubmitParams struct {
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

// SubmitApp orchestrates submission intake and dispatch.
// It is intentionally created from ServiceContext and used by logic layer directly.
type SubmitApp struct {
	submissionRepo repository.SubmissionRepository
	statusRepo     *repository.StatusRepository
	logRepo        *repository.SubmissionLogRepository
	storage        storage.ObjectStorage
	redis          *redis.Redis

	topics     svc.TopicConfig
	pushers    svc.TopicPushers
	timeouts   TimeoutConfig
	contestRpc contestrpc.ContestRpc

	contestDispatchTopic  string
	contestDispatchPusher svc.TopicPusher
	contestDispatchSwitch *svc.ContestDispatchSwitch

	sourceBucket    string
	sourceKeyPrefix string
	maxCodeBytes    int
	idempotencyTTL  time.Duration
	batchLimit      int
	rateLimit       RateLimitConfig
}

func NewSubmitApp(svcCtx *svc.ServiceContext) (*SubmitApp, error) {
	if svcCtx == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("submit app is not configured")
	}
	if svcCtx.SubmissionRepo == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("submission repository is not configured")
	}
	if svcCtx.StatusRepo == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}
	if svcCtx.Storage == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("storage is not configured")
	}
	if svcCtx.Redis == nil {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	if strings.TrimSpace(svcCtx.Config.Submit.SourceBucket) == "" {
		return nil, appErr.New(appErr.ServiceUnavailable).WithMessage("source bucket is not configured")
	}

	sourceKeyPrefix := strings.TrimSpace(svcCtx.Config.Submit.SourceKeyPrefix)
	if sourceKeyPrefix == "" {
		sourceKeyPrefix = defaultSourcePrefix
	}

	batchLimit := svcCtx.Config.Submit.BatchLimit
	if batchLimit <= 0 {
		batchLimit = defaultBatchLimit
	}

	return &SubmitApp{
		submissionRepo: svcCtx.SubmissionRepo,
		statusRepo:     svcCtx.StatusRepo,
		logRepo:        svcCtx.LogRepo,
		storage:        svcCtx.Storage,
		redis:          svcCtx.Redis,
		topics: svc.TopicConfig{
			Level0: svcCtx.Config.Topics.Level0,
			Level1: svcCtx.Config.Topics.Level1,
			Level2: svcCtx.Config.Topics.Level2,
			Level3: svcCtx.Config.Topics.Level3,
		},
		pushers:         svcCtx.TopicPushers,
		sourceBucket:    svcCtx.Config.Submit.SourceBucket,
		sourceKeyPrefix: sourceKeyPrefix,
		maxCodeBytes:    svcCtx.Config.Submit.MaxCodeBytes,
		idempotencyTTL:  svcCtx.Config.Submit.IdempotencyTTL,
		batchLimit:      batchLimit,
		rateLimit: RateLimitConfig{
			UserMax: svcCtx.Config.Submit.RateLimit.UserMax,
			IPMax:   svcCtx.Config.Submit.RateLimit.IPMax,
			Window:  svcCtx.Config.Submit.RateLimit.Window,
		},
		timeouts: TimeoutConfig{
			DB:         svcCtx.Config.Submit.Timeouts.DB,
			Cache:      svcCtx.Config.Submit.Timeouts.Cache,
			MQ:         svcCtx.Config.Submit.Timeouts.MQ,
			Storage:    svcCtx.Config.Submit.Timeouts.Storage,
			Status:     svcCtx.Config.Submit.Timeouts.Status,
			ContestRPC: svcCtx.Config.Submit.Timeouts.ContestRPC,
		},
		contestRpc:            svcCtx.ContestRpc,
		contestDispatchTopic:  svcCtx.Config.Submit.ContestDispatch.Topic,
		contestDispatchPusher: svcCtx.ContestDispatchPusher,
		contestDispatchSwitch: svcCtx.ContestDispatchSwitch,
	}, nil
}

// Submit creates a submission and dispatches it to judge queues.
func (a *SubmitApp) Submit(ctx context.Context, input SubmitParams) (string, domain.JudgeStatusPayload, error) {
	logx.WithContext(ctx).Infof("submit start problem_id=%d user_id=%d scene=%s", input.ProblemID, input.UserID, input.Scene)
	if err := a.validateInput(input); err != nil {
		return "", domain.JudgeStatusPayload{}, err
	}
	if err := a.checkRateLimit(ctx, input.UserID, input.ClientIP); err != nil {
		return "", domain.JudgeStatusPayload{}, err
	}

	acquired, existingID, err := a.acquireIdempotency(ctx, input.IdempotencyKey)
	if err != nil {
		return "", domain.JudgeStatusPayload{}, err
	}
	if !acquired && existingID != "" {
		status, statusErr := a.statusRepo.Get(ctx, existingID)
		if statusErr != nil {
			return "", domain.JudgeStatusPayload{}, statusErr
		}
		return existingID, status, nil
	}
	if err := a.checkContestEligibility(ctx, input); err != nil {
		a.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", domain.JudgeStatusPayload{}, err
	}

	submissionID := uuid.NewString()
	sourceHash := hashSource(input.SourceCode)
	sourceKey := a.buildSourceKey(submissionID)
	createdAt := time.Now()

	if err := a.uploadSource(ctx, sourceKey, input.SourceCode); err != nil {
		a.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", domain.JudgeStatusPayload{}, err
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

	if err := a.createSubmission(ctx, submission); err != nil {
		a.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", domain.JudgeStatusPayload{}, err
	}

	pending := domain.JudgeStatusPayload{
		SubmissionID: submissionID,
		Status:       domain.StatusPending,
		Timestamps:   domain.Timestamps{ReceivedAt: createdAt.Unix()},
		Progress:     domain.Progress{TotalTests: 0, DoneTests: 0},
	}
	if err := a.saveStatus(ctx, pending); err != nil {
		a.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", domain.JudgeStatusPayload{}, err
	}

	if err := a.publishMessage(ctx, submission, input.ExtraCompileFlags); err != nil {
		a.releaseIdempotency(ctx, input.IdempotencyKey, acquired)
		return "", domain.JudgeStatusPayload{}, err
	}

	a.finalizeIdempotency(ctx, input.IdempotencyKey, submissionID, acquired)
	return submissionID, pending, nil
}

func (a *SubmitApp) GetStatus(ctx context.Context, submissionID, include string) (domain.JudgeStatusPayload, error) {
	if submissionID == "" {
		return domain.JudgeStatusPayload{}, appErr.ValidationError("submission_id", "required")
	}
	ctxStatus := withTimeout(ctx, a.timeouts.Status)
	defer ctxStatus.cancel()
	status, err := a.statusRepo.Get(ctxStatus.ctx, submissionID)
	if err != nil {
		return domain.JudgeStatusPayload{}, err
	}
	if !isFinalStatus(status.Status) || strings.TrimSpace(include) == "" {
		return summaryStatus(status), nil
	}
	detail, err := a.statusRepo.GetFinalDetail(ctxStatus.ctx, submissionID)
	if err != nil {
		return domain.JudgeStatusPayload{}, err
	}
	if a.logRepo == nil {
		logx.WithContext(ctx).Error("log repository is not configured")
		return detail, nil
	}
	return a.withLogs(ctxStatus.ctx, detail)
}

func (a *SubmitApp) checkContestEligibility(ctx context.Context, input SubmitParams) error {
	if strings.TrimSpace(input.ContestID) == "" {
		return nil
	}
	if a.isContestDispatchKafka() {
		logx.WithContext(ctx).Infof("contest eligibility skip rpc due to kafka mode contest_id=%s", input.ContestID)
		return nil
	}
	if a.contestRpc == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("contest rpc is not configured")
	}
	ctxRPC := withTimeout(ctx, a.timeouts.ContestRPC)
	defer ctxRPC.cancel()
	resp, err := a.contestRpc.CheckSubmissionEligibility(ctxRPC.ctx, &contestrpc.CheckSubmissionEligibilityRequest{
		ContestId: input.ContestID,
		UserId:    input.UserID,
		ProblemId: input.ProblemID,
		SubmitAt:  time.Now().Unix(),
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("contest eligibility check failed: %v", err)
		return appErr.New(appErr.ServiceUnavailable).WithMessage("contest eligibility check failed")
	}
	if resp == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("contest eligibility response is empty")
	}
	if !resp.Ok {
		code := appErr.ErrorCode(resp.ErrorCode)
		if code == appErr.Success {
			code = appErr.ServiceUnavailable
		}
		return appErr.New(code).WithMessage(resp.ErrorMessage)
	}
	return nil
}

func (a *SubmitApp) GetStatusBatch(ctx context.Context, submissionIDs []string) ([]domain.JudgeStatusPayload, []string, error) {
	if len(submissionIDs) == 0 {
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	if len(submissionIDs) > a.batchLimit {
		return nil, nil, appErr.ValidationError("submission_ids", "too_many")
	}
	ctxStatus := withTimeout(ctx, a.timeouts.Status)
	defer ctxStatus.cancel()
	return a.statusRepo.GetBatch(ctxStatus.ctx, submissionIDs)
}

func (a *SubmitApp) GetSource(ctx context.Context, submissionID string) (*repository.Submission, error) {
	if submissionID == "" {
		return nil, appErr.ValidationError("submission_id", "required")
	}
	ctxDB := withTimeout(ctx, a.timeouts.DB)
	defer ctxDB.cancel()
	submission, err := a.submissionRepo.GetByID(ctxDB.ctx, nil, submissionID)
	if err != nil {
		if errors.Is(err, repository.ErrSubmissionNotFound) {
			return nil, appErr.New(appErr.SubmissionNotFound).WithMessage("submission not found")
		}
		return nil, appErr.Wrapf(err, appErr.DatabaseError, "get submission failed")
	}
	return submission, nil
}

func (a *SubmitApp) withLogs(ctx context.Context, status domain.JudgeStatusPayload) (domain.JudgeStatusPayload, error) {
	if status.Compile != nil {
		if status.Compile.Log == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeCompileLog, ""); err == nil {
				status.Compile.Log = logItem.Content
			}
		}
		if status.Compile.Error == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeCompileError, ""); err == nil {
				status.Compile.Error = logItem.Content
			}
		}
	}
	if len(status.Tests) == 0 {
		return status, nil
	}
	tests := make([]domain.TestcaseResult, 0, len(status.Tests))
	for _, test := range status.Tests {
		item := test
		if item.RuntimeLog == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeRuntime, item.TestID); err == nil {
				item.RuntimeLog = logItem.Content
			}
		}
		if item.CheckerLog == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeChecker, item.TestID); err == nil {
				item.CheckerLog = logItem.Content
			}
		}
		if item.Stdout == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeStdout, item.TestID); err == nil {
				item.Stdout = logItem.Content
			}
		}
		if item.Stderr == "" {
			if logItem, err := a.logRepo.Get(ctx, status.SubmissionID, repository.LogTypeStderr, item.TestID); err == nil {
				item.Stderr = logItem.Content
			}
		}
		tests = append(tests, item)
	}
	status.Tests = tests
	return status, nil
}

func summaryStatus(status domain.JudgeStatusPayload) domain.JudgeStatusPayload {
	summary := status
	summary.Compile = nil
	summary.Tests = nil
	return summary
}

func isFinalStatus(status string) bool {
	return status == domain.StatusFinished || status == domain.StatusFailed
}

func (a *SubmitApp) validateInput(input SubmitParams) error {
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
	if a.maxCodeBytes > 0 && len([]byte(input.SourceCode)) > a.maxCodeBytes {
		return appErr.New(appErr.CodeTooLarge).WithMessage("source code too large")
	}
	return nil
}

func (a *SubmitApp) acquireIdempotency(ctx context.Context, key string) (bool, string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return true, "", nil
	}
	cacheKey := idempotencyKeyPrefix + key
	ctxCache := withTimeout(ctx, a.timeouts.Cache)
	defer ctxCache.cancel()

	existing, err := a.redis.GetCtx(ctxCache.ctx, cacheKey)
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "read idempotency key failed")
	}
	if existing != "" && existing != processingMarker {
		return false, existing, nil
	}

	ttl := a.idempotencyTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	ok, err := a.redis.SetnxExCtx(ctxCache.ctx, cacheKey, processingMarker, ttlSeconds(ttl))
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "reserve idempotency key failed")
	}
	if ok {
		return true, "", nil
	}
	existing, err = a.redis.GetCtx(ctxCache.ctx, cacheKey)
	if err != nil {
		return false, "", appErr.Wrapf(err, appErr.CacheError, "read idempotency key failed")
	}
	if existing != "" && existing != processingMarker {
		return false, existing, nil
	}
	return false, "", appErr.New(appErr.TooManyRequests).WithMessage("request is processing")
}

func (a *SubmitApp) finalizeIdempotency(ctx context.Context, key, submissionID string, acquired bool) {
	if !acquired || strings.TrimSpace(key) == "" {
		return
	}
	cacheKey := idempotencyKeyPrefix + strings.TrimSpace(key)
	ttl := a.idempotencyTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	ctxCache := withTimeout(ctx, a.timeouts.Cache)
	defer ctxCache.cancel()
	if err := a.redis.SetexCtx(ctxCache.ctx, cacheKey, submissionID, ttlSeconds(ttl)); err != nil {
		logx.WithContext(ctx).Errorf("update idempotency key failed: %v", err)
	}
}

func (a *SubmitApp) releaseIdempotency(ctx context.Context, key string, acquired bool) {
	if !acquired || strings.TrimSpace(key) == "" {
		return
	}
	cacheKey := idempotencyKeyPrefix + strings.TrimSpace(key)
	ctxCache := withTimeout(ctx, a.timeouts.Cache)
	defer ctxCache.cancel()
	if _, err := a.redis.DelCtx(ctxCache.ctx, cacheKey); err != nil {
		logx.WithContext(ctx).Errorf("release idempotency key failed: %v", err)
	}
}

func (a *SubmitApp) checkRateLimit(ctx context.Context, userID int64, clientIP string) error {
	if a.rateLimit.Window <= 0 || (a.rateLimit.UserMax <= 0 && a.rateLimit.IPMax <= 0) {
		return nil
	}
	ctxCache := withTimeout(ctx, a.timeouts.Cache)
	defer ctxCache.cancel()

	if a.rateLimit.UserMax > 0 && userID > 0 {
		if err := a.checkRateCounter(ctxCache.ctx, rateUserKeyPrefix+fmt.Sprintf("%d", userID), a.rateLimit.UserMax); err != nil {
			return err
		}
	}
	if a.rateLimit.IPMax > 0 && clientIP != "" {
		if err := a.checkRateCounter(ctxCache.ctx, rateIPKeyPrefix+clientIP, a.rateLimit.IPMax); err != nil {
			return err
		}
	}
	return nil
}

func (a *SubmitApp) checkRateCounter(ctx context.Context, key string, max int) error {
	count, err := a.redis.IncrCtx(ctx, key)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "rate limit check failed")
	}
	if count == 1 {
		_ = a.redis.ExpireCtx(ctx, key, ttlSeconds(a.rateLimit.Window))
	}
	if int(count) > max {
		return appErr.New(appErr.SubmitTooFrequently).WithMessage("submit too frequently")
	}
	return nil
}

func (a *SubmitApp) uploadSource(ctx context.Context, objectKey, source string) error {
	sizeBytes := int64(len([]byte(source)))
	reader := io.NopCloser(strings.NewReader(source))
	defer reader.Close()
	ctxStorage := withTimeout(ctx, a.timeouts.Storage)
	defer ctxStorage.cancel()
	if err := a.storage.PutObject(ctxStorage.ctx, a.sourceBucket, objectKey, reader, sizeBytes, "text/plain; charset=utf-8"); err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "upload source failed")
	}
	return nil
}

func (a *SubmitApp) createSubmission(ctx context.Context, submission *repository.Submission) error {
	ctxDB := withTimeout(ctx, a.timeouts.DB)
	defer ctxDB.cancel()
	if err := a.submissionRepo.Create(ctxDB.ctx, nil, submission); err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "create submission failed")
	}
	return nil
}

func (a *SubmitApp) saveStatus(ctx context.Context, status domain.JudgeStatusPayload) error {
	ctxStatus := withTimeout(ctx, a.timeouts.Status)
	defer ctxStatus.cancel()
	if err := a.statusRepo.Save(ctxStatus.ctx, status); err != nil {
		return err
	}
	return nil
}

func (a *SubmitApp) publishMessage(ctx context.Context, submission *repository.Submission, extraFlags []string) error {
	payload := domain.JudgeMessage{
		SubmissionID:      submission.SubmissionID,
		ProblemID:         submission.ProblemID,
		LanguageID:        submission.LanguageID,
		SourceKey:         submission.SourceKey,
		SourceHash:        submission.SourceHash,
		ContestID:         submission.ContestID,
		UserID:            fmt.Sprintf("%d", submission.UserID),
		Priority:          resolvePriority(submission.Scene),
		ExtraCompileFlags: extraFlags,
		CreatedAt:         time.Now().Unix(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "encode judge message failed")
	}

	if a.shouldDispatchContest(submission) {
		return a.publishContestDispatch(ctx, submission, string(body))
	}

	topic := resolveTopic(submission.Scene, a.topics)
	pusher := a.pusherForTopic(topic)
	if pusher == nil {
		return appErr.New(appErr.SubmissionCreateFailed).WithMessage("judge topic is not configured")
	}
	ctxMQ := withTimeout(ctx, a.timeouts.MQ)
	defer ctxMQ.cancel()
	if err := pusher.PushWithKey(ctxMQ.ctx, submission.SubmissionID, string(body)); err != nil {
		logx.WithContext(ctxMQ.ctx).Errorf(
			"publish judge message failed: %v topic=%s submission_id=%s problem_id=%d user_id=%d scene=%s",
			err,
			topic,
			submission.SubmissionID,
			submission.ProblemID,
			submission.UserID,
			submission.Scene,
		)
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "publish judge message failed")
	}
	logx.WithContext(ctxMQ.ctx).Infof(
		"publish judge message ok topic=%s submission_id=%s problem_id=%d user_id=%d scene=%s",
		topic,
		submission.SubmissionID,
		submission.ProblemID,
		submission.UserID,
		submission.Scene,
	)
	return nil
}

func (a *SubmitApp) publishContestDispatch(ctx context.Context, submission *repository.Submission, body string) error {
	if a.contestDispatchPusher == nil || strings.TrimSpace(a.contestDispatchTopic) == "" {
		return appErr.New(appErr.SubmissionCreateFailed).WithMessage("contest dispatch topic is not configured")
	}
	ctxMQ := withTimeout(ctx, a.timeouts.MQ)
	defer ctxMQ.cancel()
	if err := a.contestDispatchPusher.PushWithKey(ctxMQ.ctx, submission.SubmissionID, body); err != nil {
		logx.WithContext(ctxMQ.ctx).Errorf(
			"publish contest dispatch failed: %v topic=%s submission_id=%s problem_id=%d user_id=%d scene=%s",
			err,
			a.contestDispatchTopic,
			submission.SubmissionID,
			submission.ProblemID,
			submission.UserID,
			submission.Scene,
		)
		return appErr.Wrapf(err, appErr.SubmissionCreateFailed, "publish contest dispatch failed")
	}
	logx.WithContext(ctxMQ.ctx).Infof(
		"publish contest dispatch ok topic=%s submission_id=%s problem_id=%d user_id=%d scene=%s",
		a.contestDispatchTopic,
		submission.SubmissionID,
		submission.ProblemID,
		submission.UserID,
		submission.Scene,
	)
	return nil
}

func (a *SubmitApp) shouldDispatchContest(submission *repository.Submission) bool {
	if submission == nil {
		return false
	}
	if strings.TrimSpace(submission.ContestID) == "" {
		return false
	}
	return a.isContestDispatchKafka()
}

func (a *SubmitApp) isContestDispatchKafka() bool {
	if a.contestDispatchSwitch == nil {
		return false
	}
	return a.contestDispatchSwitch.Mode() == svc.ContestDispatchModeKafka
}

func (a *SubmitApp) pusherForTopic(topic string) svc.TopicPusher {
	switch topic {
	case a.topics.Level0:
		return a.pushers.Level0
	case a.topics.Level1:
		return a.pushers.Level1
	case a.topics.Level2:
		return a.pushers.Level2
	case a.topics.Level3:
		return a.pushers.Level3
	default:
		return nil
	}
}

func (a *SubmitApp) buildSourceKey(submissionID string) string {
	return fmt.Sprintf("%s/%s/source.code", a.sourceKeyPrefix, submissionID)
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

func resolveTopic(scene string, topics svc.TopicConfig) string {
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

func ttlSeconds(ttl time.Duration) int {
	if ttl <= 0 {
		return 0
	}
	seconds := int(ttl.Seconds())
	if seconds <= 0 {
		return 1
	}
	return seconds
}
