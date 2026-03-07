package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fuzoj/pkg/contest/eligibility"
	contestRepo "fuzoj/pkg/contest/repository"
	"fuzoj/pkg/contest/score"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/pmodel"
	"fuzoj/services/contest_service/internal/repository"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	rankIdemKeyPrefix = "contest:rank:idem:"
	rankResultIDKey   = "contest:rank:result:id:"
)

// JudgeFinalConsumer processes final judge status messages for contest ranking.
type JudgeFinalConsumer struct {
	conn              sqlx.SqlConn
	redis             *redis.Redis
	contestRepo       contestRepo.ContestRepository
	eligibilitySvc    *eligibility.Service
	memberProblemRepo *repository.MemberProblemRepository
	memberSummaryRepo *repository.MemberSummaryRepository
	outboxRepo        *repository.RankOutboxRepository
	deadLetterPusher  *kq.Pusher
	opts              JudgeFinalOptions
	timeouts          TimeoutConfig
}

// JudgeFinalOptions holds consumer options.
type JudgeFinalOptions struct {
	IdempotencyTTL  time.Duration
	MessageTTL      time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	DeadLetterTopic string
}

// NewJudgeFinalConsumer creates a judge final status consumer.
func NewJudgeFinalConsumer(
	conn sqlx.SqlConn,
	redisClient *redis.Redis,
	contestRepository contestRepo.ContestRepository,
	eligibilityService *eligibility.Service,
	memberProblemRepo *repository.MemberProblemRepository,
	memberSummaryRepo *repository.MemberSummaryRepository,
	outboxRepo *repository.RankOutboxRepository,
	opts JudgeFinalOptions,
	timeouts TimeoutConfig,
) *JudgeFinalConsumer {
	if opts.IdempotencyTTL <= 0 {
		opts.IdempotencyTTL = 30 * time.Minute
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.RetryDelay <= 0 {
		opts.RetryDelay = time.Second
	}
	return &JudgeFinalConsumer{
		conn:              conn,
		redis:             redisClient,
		contestRepo:       contestRepository,
		eligibilitySvc:    eligibilityService,
		memberProblemRepo: memberProblemRepo,
		memberSummaryRepo: memberSummaryRepo,
		outboxRepo:        outboxRepo,
		opts:              opts,
		timeouts:          timeouts,
	}
}

// SetDeadLetterPusher configures the dead-letter pusher.
func (c *JudgeFinalConsumer) SetDeadLetterPusher(pusher *kq.Pusher) {
	c.deadLetterPusher = pusher
}

// Consume handles final status messages.
func (c *JudgeFinalConsumer) Consume(ctx context.Context, key, value string) error {
	if value == "" {
		return nil
	}
	logger := logx.WithContext(ctx)
	if c == nil || c.conn == nil || c.contestRepo == nil || c.eligibilitySvc == nil ||
		c.memberProblemRepo == nil || c.memberSummaryRepo == nil || c.outboxRepo == nil {
		logger.Error("judge final consumer is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge final consumer is not configured")
	}

	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if err := c.handle(ctx, key, value); err == nil {
			return nil
		} else if attempt >= c.opts.MaxRetries {
			if c.opts.DeadLetterTopic != "" && c.deadLetterPusher != nil {
				_ = c.deadLetterPusher.PushWithKey(ctx, key, value)
			}
			logger.Errorf("judge final consume failed after retries: %v", err)
			return nil
		}
		time.Sleep(c.opts.RetryDelay)
	}
	return nil
}

func (c *JudgeFinalConsumer) handle(ctx context.Context, key, value string) error {
	logger := logx.WithContext(ctx)
	var event pmodel.StatusEvent
	if err := json.Unmarshal([]byte(value), &event); err != nil {
		logger.Errorf("decode judge final event failed: %v", err)
		return appErr.Wrapf(err, appErr.InvalidParams, "decode judge final event failed")
	}
	if event.Type != pmodel.StatusEventFinal {
		return nil
	}
	status := event.Status
	if status.SubmissionID == "" || status.ContestID == "" || strings.TrimSpace(status.UserID) == "" || status.ProblemID <= 0 {
		return nil
	}

	if c.opts.MessageTTL > 0 && event.CreatedAt > 0 {
		if time.Since(time.Unix(event.CreatedAt, 0)) > c.opts.MessageTTL {
			logger.Infof("judge final event expired submission_id=%s", status.SubmissionID)
			return nil
		}
	}

	if c.redis != nil {
		finishedAt := status.Timestamps.FinishedAt
		if finishedAt <= 0 {
			finishedAt = event.CreatedAt
		}
		idemKey := rankIdemKeyPrefix + status.SubmissionID + ":" + fmt.Sprint(finishedAt)
		ok, err := c.redis.SetnxExCtx(ctx, idemKey, "1", ttlSeconds(c.opts.IdempotencyTTL))
		if err != nil {
			logger.Errorf("judge final idempotency failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "judge final idempotency failed")
		}
		if !ok {
			return nil
		}
	}

	userID, err := strconv.ParseInt(status.UserID, 10, 64)
	if err != nil || userID <= 0 {
		return nil
	}
	submitAt := submissionTime(status)
	ctxMQ := withTimeout(ctx, c.timeouts.MQ)
	defer ctxMQ.cancel()

	result, err := c.eligibilitySvc.Check(ctxMQ.ctx, eligibility.Request{
		ContestID: status.ContestID,
		UserID:    userID,
		ProblemID: status.ProblemID,
		Now:       submitAt,
	})
	if err != nil {
		logger.Errorf("contest eligibility check failed: %v", err)
		return err
	}
	if !result.OK {
		return nil
	}

	meta, err := c.contestRepo.GetMeta(ctxMQ.ctx, status.ContestID)
	if err != nil {
		logger.Errorf("load contest meta failed: %v", err)
		return err
	}

	resultID, err := c.nextResultID(ctx, status.ContestID)
	if err != nil {
		logger.Errorf("generate contest result id failed: %v", err)
		return err
	}

	return c.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		memberProblemRepo := repository.NewMemberProblemRepository(session)
		memberSummaryRepo := repository.NewMemberSummaryRepository(session)
		outboxRepo := repository.NewRankOutboxRepository(session)

		memberID := status.UserID
		problemID := status.ProblemID

		state, found, err := memberProblemRepo.Get(ctx, status.ContestID, memberID, problemID)
		if err != nil {
			return err
		}
		if found && state.Solved {
			return nil
		}

		isAC := strings.EqualFold(status.Verdict, "AC")
		if !found {
			state = repository.MemberProblemState{
				ContestID: status.ContestID,
				MemberID:  memberID,
				ProblemID: problemID,
			}
		}
		if !isAC {
			state.WrongCount++
		}
		state.LastSubmissionID = status.SubmissionID
		state.LastSubmissionAt = submitAt
		if isAC {
			state.Solved = true
			state.FirstACAt = submitAt
			state.Score = 1
			state.Penalty = score.ICPCPenalty(meta.StartAt, submitAt, state.WrongCount)
		}
		state.UpdatedAt = time.Now()

		if err := memberProblemRepo.Upsert(ctx, state); err != nil {
			return err
		}

		summary, summaryFound, err := memberSummaryRepo.Get(ctx, status.ContestID, memberID)
		if err != nil {
			return err
		}
		if !summaryFound {
			summary = repository.MemberSummarySnapshot{
				ContestID: status.ContestID,
				MemberID:  memberID,
			}
		}

		detail := parseMemberDetail(summary.DetailJSON)
		if detail.Problems == nil {
			detail.Problems = make(map[string]ProblemDetail)
		}
		problemKey := fmt.Sprint(problemID)
		detail.Problems[problemKey] = ProblemDetail{
			Solved:           state.Solved,
			WrongCount:       state.WrongCount,
			FirstACAt:        unixTime(state.FirstACAt),
			LastSubmissionAt: unixTime(state.LastSubmissionAt),
			LastSubmissionID: state.LastSubmissionID,
			Penalty:          state.Penalty,
			Verdict:          status.Verdict,
		}
		detail.UpdatedAt = time.Now().Unix()

		if isAC {
			summary.ACCount++
			summary.ScoreTotal++
			summary.PenaltyTotal += state.Penalty
		}
		summary.Version++
		summary.DetailJSON = mustMarshalDetail(detail)
		summary.UpdatedAt = time.Now()

		if err := memberSummaryRepo.Upsert(ctx, summary); err != nil {
			return err
		}

		update := pmodel.RankUpdateEvent{
			ContestID:  status.ContestID,
			MemberID:   memberID,
			ProblemID:  problemKey,
			SortScore:  score.SortScore(summary.ScoreTotal, summary.PenaltyTotal),
			ScoreTotal: summary.ScoreTotal,
			Penalty:    summary.PenaltyTotal,
			ACCount:    summary.ACCount,
			DetailJSON: summary.DetailJSON,
			Version:    fmt.Sprint(summary.Version),
			ResultID:   resultID,
			UpdatedAt:  summary.UpdatedAt.Unix(),
		}
		payload, err := json.Marshal(update)
		if err != nil {
			return fmt.Errorf("marshal rank update failed: %w", err)
		}
		eventKey := status.ContestID + ":" + memberID + ":" + update.Version
		kafkaKey := status.ContestID
		if err := outboxRepo.Enqueue(ctx, repository.RankOutboxEvent{
			EventKey: eventKey,
			KafkaKey: kafkaKey,
			Payload:  string(payload),
		}); err != nil {
			return err
		}
		return nil
	})
}

func (c *JudgeFinalConsumer) nextResultID(ctx context.Context, contestID string) (int64, error) {
	if contestID == "" {
		return 0, appErr.ValidationError("contest_id", "required")
	}
	if c.redis == nil {
		return 0, appErr.New(appErr.ServiceUnavailable).WithMessage("redis is not configured")
	}
	ctxCache := withTimeout(ctx, c.timeouts.Cache)
	defer ctxCache.cancel()
	id, err := c.redis.IncrCtx(ctxCache.ctx, rankResultIDKey+contestID)
	if err != nil {
		return 0, appErr.Wrapf(err, appErr.CacheError, "increment contest result id failed")
	}
	return id, nil
}

// ProblemDetail holds per-problem detail for a member.
type ProblemDetail struct {
	Solved           bool   `json:"solved"`
	WrongCount       int    `json:"wrong_count"`
	FirstACAt        int64  `json:"first_ac_at"`
	LastSubmissionAt int64  `json:"last_submission_at"`
	LastSubmissionID string `json:"last_submission_id"`
	Penalty          int64  `json:"penalty"`
	Verdict          string `json:"verdict"`
}

// MemberDetail holds aggregated detail for a member.
type MemberDetail struct {
	Problems  map[string]ProblemDetail `json:"problems"`
	UpdatedAt int64                    `json:"updated_at"`
}

func parseMemberDetail(raw string) MemberDetail {
	if raw == "" {
		return MemberDetail{Problems: map[string]ProblemDetail{}}
	}
	var detail MemberDetail
	if err := json.Unmarshal([]byte(raw), &detail); err != nil {
		return MemberDetail{Problems: map[string]ProblemDetail{}}
	}
	if detail.Problems == nil {
		detail.Problems = map[string]ProblemDetail{}
	}
	return detail
}

func mustMarshalDetail(detail MemberDetail) string {
	data, err := json.Marshal(detail)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func unixTime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func submissionTime(status pmodel.JudgeStatusResponse) time.Time {
	if status.CreatedAt > 0 {
		return time.Unix(status.CreatedAt, 0)
	}
	if status.Timestamps.ReceivedAt > 0 {
		return time.Unix(status.Timestamps.ReceivedAt, 0)
	}
	return time.Now()
}
