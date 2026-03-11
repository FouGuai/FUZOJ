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
	rankUpdatePusher  *kq.Pusher
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
	_ *repository.MemberProblemRepository,
	_ *repository.MemberSummaryRepository,
	_ *repository.RankOutboxRepository,
	rankUpdatePusher *kq.Pusher,
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
		rankUpdatePusher:  rankUpdatePusher,
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
	if c == nil || c.conn == nil || c.contestRepo == nil || c.eligibilitySvc == nil || c.rankUpdatePusher == nil {
		logger.Error("judge final consumer is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge final consumer is not configured")
	}

	for attempt := 0; attempt <= c.opts.MaxRetries; attempt++ {
		if err := c.handle(ctx, key, value); err == nil {
			return nil
		} else if attempt >= c.opts.MaxRetries {
			if c.opts.DeadLetterTopic != "" && c.deadLetterPusher != nil {
				if dlqErr := c.deadLetterPusher.PushWithKey(ctx, key, value); dlqErr != nil {
					logger.Errorf("judge final consume failed and dead-letter publish failed: consume_err=%v dlq_err=%v", err, dlqErr)
					return err
				}
				logger.Errorf("judge final consume failed after retries, message moved to dead-letter: %v", err)
				return nil
			}
			logger.Errorf("judge final consume failed after retries: %v", err)
			return err
		}
		time.Sleep(c.opts.RetryDelay)
	}
	return nil
}

func (c *JudgeFinalConsumer) handle(ctx context.Context, key, value string) (retErr error) {
	logger := logx.WithContext(ctx)
	var (
		idemKey   string
		idemKeyOk bool
	)
	defer func() {
		if !idemKeyOk || retErr == nil || c.redis == nil {
			return
		}
		ctxCache := withTimeout(context.Background(), c.timeouts.Cache)
		if _, delErr := c.redis.DelCtx(ctxCache.ctx, idemKey); delErr != nil {
			logger.Errorf("clear judge final idempotency key failed: key=%s err=%v", idemKey, delErr)
		}
		ctxCache.cancel()
	}()

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
		idemKey = rankIdemKeyPrefix + status.SubmissionID + ":" + fmt.Sprint(finishedAt)
		ok, err := c.redis.SetnxExCtx(ctx, idemKey, "1", ttlSeconds(c.opts.IdempotencyTTL))
		if err != nil {
			logger.Errorf("judge final idempotency failed: %v", err)
			return appErr.Wrapf(err, appErr.CacheError, "judge final idempotency failed")
		}
		if !ok {
			return nil
		}
		idemKeyOk = true
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

	var update pmodel.RankUpdateEvent
	err = c.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		memberProblemRepo := repository.NewMemberProblemRepository(session)
		memberSummaryRepo := repository.NewMemberSummaryRepository(session)

		memberID := status.UserID
		problemID := status.ProblemID
		problemKey := fmt.Sprint(problemID)

		state, found, err := memberProblemRepo.Get(ctx, status.ContestID, memberID, problemID)
		if err != nil {
			return err
		}
		if found && state.Solved {
			summary, summaryFound, err := memberSummaryRepo.Get(ctx, status.ContestID, memberID)
			if err != nil {
				return err
			}
			if !summaryFound {
				return fmt.Errorf("member summary not found for solved state, contest=%s member=%s", status.ContestID, memberID)
			}
			update = pmodel.RankUpdateEvent{
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

		update = pmodel.RankUpdateEvent{
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
		return nil
	})
	if err != nil {
		return err
	}

	payload, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("marshal rank update failed: %w", err)
	}
	ctxPush := withTimeout(ctx, c.timeouts.MQ)
	defer ctxPush.cancel()
	if err := c.rankUpdatePusher.PushWithKey(ctxPush.ctx, status.ContestID, string(payload)); err != nil {
		logger.Errorf("push rank update failed contest=%s member=%s result_id=%d err=%v", status.ContestID, status.UserID, resultID, err)
		return fmt.Errorf("push rank update failed: %w", err)
	}

	return nil
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
