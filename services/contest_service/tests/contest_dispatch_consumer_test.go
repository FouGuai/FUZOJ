package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"fuzoj/pkg/contest/eligibility"
	contestRepo "fuzoj/pkg/contest/repository"
	"fuzoj/pkg/submit/statuswriter"
	"fuzoj/services/contest_service/internal/consumer"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestContestDispatchConsumer(t *testing.T) {
	t.Run("eligibility ok forwards to judge", func(t *testing.T) {
		redisServer, redisClient := newTestRedis(t)
		_ = redisServer

		conn := &fakeSqlConn{rows: 1}
		writer := statuswriter.NewFinalStatusWriter(conn, redisClient, time.Minute)

		repos := newEligibilityRepos(true, "approved")
		service := eligibility.NewService(repos.contest, repos.problem, repos.participant)

		judgePusher := &fakePusher{}
		consumer := consumer.NewContestDispatchConsumer(service, writer, redisClient, judgePusher, consumer.DispatchOptions{
			IdempotencyTTL: time.Minute,
			MaxRetries:     0,
		}, consumer.TimeoutConfig{MQ: time.Second})

		payload := contestDispatchMessage{
			SubmissionID: "sub-1",
			ProblemID:    100,
			ContestID:    "contest-1",
			UserID:       "200",
			CreatedAt:    time.Now().Unix(),
		}
		body := mustJSON(payload)
		if err := consumer.Consume(context.Background(), "sub-1", body); err != nil {
			t.Fatalf("consume failed: %v", err)
		}
		if len(judgePusher.keys) != 1 {
			t.Fatalf("expected judge pusher to be called")
		}
		if conn.execCalls != 0 {
			t.Fatalf("unexpected db writes: %d", conn.execCalls)
		}
	})

	t.Run("eligibility rejected writes final status", func(t *testing.T) {
		redisServer, redisClient := newTestRedis(t)
		_ = redisServer

		conn := &fakeSqlConn{rows: 1}
		writer := statuswriter.NewFinalStatusWriter(conn, redisClient, time.Minute)

		repos := newEligibilityRepos(false, "denied")
		service := eligibility.NewService(repos.contest, repos.problem, repos.participant)

		judgePusher := &fakePusher{}
		consumer := consumer.NewContestDispatchConsumer(service, writer, redisClient, judgePusher, consumer.DispatchOptions{
			IdempotencyTTL: time.Minute,
			MaxRetries:     0,
		}, consumer.TimeoutConfig{MQ: time.Second})

		payload := contestDispatchMessage{
			SubmissionID: "sub-2",
			ProblemID:    100,
			ContestID:    "contest-1",
			UserID:       "200",
			CreatedAt:    time.Now().Unix(),
		}
		body := mustJSON(payload)
		if err := consumer.Consume(context.Background(), "sub-2", body); err != nil {
			t.Fatalf("consume failed: %v", err)
		}
		if len(judgePusher.keys) != 0 {
			t.Fatalf("unexpected judge pusher calls: %d", len(judgePusher.keys))
		}
		if conn.execCalls != 1 {
			t.Fatalf("expected final status to be stored")
		}
		if conn.lastQuery == "" {
			t.Fatalf("expected exec query to be set")
		}
	})

	t.Run("idempotency prevents duplicate", func(t *testing.T) {
		redisServer, redisClient := newTestRedis(t)
		_ = redisServer

		conn := &fakeSqlConn{rows: 1}
		writer := statuswriter.NewFinalStatusWriter(conn, redisClient, time.Minute)

		repos := newEligibilityRepos(true, "approved")
		service := eligibility.NewService(repos.contest, repos.problem, repos.participant)

		judgePusher := &fakePusher{}
		consumer := consumer.NewContestDispatchConsumer(service, writer, redisClient, judgePusher, consumer.DispatchOptions{
			IdempotencyTTL: time.Minute,
			MaxRetries:     0,
		}, consumer.TimeoutConfig{MQ: time.Second})

		payload := contestDispatchMessage{
			SubmissionID: "sub-3",
			ProblemID:    100,
			ContestID:    "contest-1",
			UserID:       "200",
			CreatedAt:    time.Now().Unix(),
		}
		body := mustJSON(payload)
		if err := consumer.Consume(context.Background(), "sub-3", body); err != nil {
			t.Fatalf("consume failed: %v", err)
		}
		if err := consumer.Consume(context.Background(), "sub-3", body); err != nil {
			t.Fatalf("consume repeat failed: %v", err)
		}
		if len(judgePusher.keys) != 1 {
			t.Fatalf("expected judge pusher called once")
		}
	})
}

type eligibilityRepos struct {
	contest     contestRepo.ContestRepository
	problem     contestRepo.ContestProblemRepository
	participant contestRepo.ContestParticipantRepository
}

func newEligibilityRepos(problemExists bool, participantStatus string) eligibilityRepos {
	return eligibilityRepos{
		contest:     fakeContestRepo{meta: contestRepo.ContestMeta{ContestID: "contest-1", StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)}},
		problem:     fakeProblemRepo{exists: problemExists},
		participant: fakeParticipantRepo{status: participantStatus},
	}
}

type fakeContestRepo struct {
	meta contestRepo.ContestMeta
}

func (r fakeContestRepo) GetMeta(ctx context.Context, contestID string) (contestRepo.ContestMeta, error) {
	if contestID == "" {
		return contestRepo.ContestMeta{}, errors.New("contestID is required")
	}
	if contestID != r.meta.ContestID {
		return contestRepo.ContestMeta{}, contestRepo.ErrContestNotFound
	}
	return r.meta, nil
}

func (r fakeContestRepo) InvalidateMetaCache(ctx context.Context, contestID string) error {
	return nil
}

type fakeProblemRepo struct {
	exists bool
}

func (r fakeProblemRepo) HasProblem(ctx context.Context, contestID string, problemID int64) (bool, error) {
	if !r.exists {
		return false, contestRepo.ErrContestProblemNotFound
	}
	return true, nil
}

func (r fakeProblemRepo) InvalidateProblemCache(ctx context.Context, contestID string, problemID int64) error {
	return nil
}

type fakeParticipantRepo struct {
	status string
}

func (r fakeParticipantRepo) GetParticipant(ctx context.Context, contestID string, userID int64) (contestRepo.ContestParticipant, error) {
	if r.status == "" {
		return contestRepo.ContestParticipant{}, contestRepo.ErrParticipantNotFound
	}
	return contestRepo.ContestParticipant{ContestID: contestID, UserID: userID, Status: r.status}, nil
}

func (r fakeParticipantRepo) InvalidateParticipantCache(ctx context.Context, contestID string, userID int64) error {
	return nil
}

type fakePusher struct {
	keys   []string
	values []string
}

func (f *fakePusher) PushWithKey(ctx context.Context, key, value string) error {
	f.keys = append(f.keys, key)
	f.values = append(f.values, value)
	return nil
}

func (f *fakePusher) Close() error {
	return nil
}

type fakeSqlConn struct {
	execCalls int
	rows      int64
	lastQuery string
}

func (c *fakeSqlConn) Exec(query string, args ...any) (sql.Result, error) {
	return c.ExecCtx(context.Background(), query, args...)
}

func (c *fakeSqlConn) ExecCtx(ctx context.Context, query string, args ...any) (sql.Result, error) {
	c.execCalls++
	c.lastQuery = query
	return fakeSQLResult{rows: c.rows}, nil
}

func (c *fakeSqlConn) Prepare(query string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeSqlConn) PrepareCtx(ctx context.Context, query string) (sqlx.StmtSession, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRow(v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowCtx(ctx context.Context, v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowPartial(v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowPartialCtx(ctx context.Context, v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRows(v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowsCtx(ctx context.Context, v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowsPartial(v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) QueryRowsPartialCtx(ctx context.Context, v any, query string, args ...any) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) RawDB() (*sql.DB, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeSqlConn) Transact(fn func(sqlx.Session) error) error {
	return errors.New("not implemented")
}

func (c *fakeSqlConn) TransactCtx(ctx context.Context, fn func(context.Context, sqlx.Session) error) error {
	return errors.New("not implemented")
}

type fakeSQLResult struct {
	rows int64
}

func (r fakeSQLResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r fakeSQLResult) RowsAffected() (int64, error) {
	return r.rows, nil
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

type contestDispatchMessage struct {
	SubmissionID string `json:"submission_id"`
	ProblemID    int64  `json:"problem_id"`
	ContestID    string `json:"contest_id"`
	UserID       string `json:"user_id"`
	CreatedAt    int64  `json:"created_at"`
}

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Redis) {
	server := miniredis.RunT(t)
	client := redis.MustNewRedis(redis.RedisConf{
		Host: server.Addr(),
		Type: "node",
	})
	t.Cleanup(func() {
		server.Close()
	})
	return server, client
}
