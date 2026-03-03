package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fuzoj/internal/common/storage"
	"fuzoj/services/submit_service/internal/config"
	"fuzoj/services/submit_service/internal/repository"
	"fuzoj/services/submit_service/internal/svc"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type errorResponse struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	TraceId string            `json:"trace_id,omitempty"`
}

func newTestServiceContext(
	cfg config.Config,
	submissionRepo repository.SubmissionRepository,
	statusRepo *repository.StatusRepository,
	logRepo *repository.SubmissionLogRepository,
	storageClient storage.ObjectStorage,
	redisClient *redis.Redis,
	pushers svc.TopicPushers,
	contestDispatchPusher svc.TopicPusher,
	contestDispatchMode string,
) *svc.ServiceContext {
	if strings.TrimSpace(contestDispatchMode) == "" {
		contestDispatchMode = svc.ContestDispatchModeRPC
	}
	return &svc.ServiceContext{
		Config:                cfg,
		SubmissionRepo:        submissionRepo,
		StatusRepo:            statusRepo,
		LogRepo:               logRepo,
		Storage:               storageClient,
		Redis:                 redisClient,
		TopicPushers:          pushers,
		ContestDispatchPusher: contestDispatchPusher,
		ContestDispatchSwitch: svc.NewContestDispatchSwitch(contestDispatchMode),
	}
}

func defaultTestConfig() config.Config {
	return config.Config{
		Topics: config.TopicConfig{
			Level0: "judge-level-0",
			Level1: "judge-level-1",
			Level2: "judge-level-2",
			Level3: "judge-level-3",
		},
		Submit: config.SubmitConfig{
			SourceBucket:    "submit-bucket",
			SourceKeyPrefix: "submissions",
			BatchLimit:      200,
			IdempotencyTTL:  10 * time.Minute,
			RateLimit: config.RateLimitConfig{
				UserMax: 0,
				IPMax:   0,
				Window:  0,
			},
			Timeouts: config.TimeoutConfig{
				DB:      time.Second,
				Cache:   time.Second,
				MQ:      time.Second,
				Storage: time.Second,
				Status:  time.Second,
			},
		},
	}
}

func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Redis) {
	mr := miniredis.RunT(t)
	client := redis.MustNewRedis(redis.RedisConf{
		Host: mr.Addr(),
		Type: "node",
	})
	t.Cleanup(func() {
		mr.Close()
	})
	return mr, client
}

func doRequest(t *testing.T, handler http.HandlerFunc, method, path string, body any, headers map[string]string, pathVars map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	switch val := body.(type) {
	case nil:
		reader = nil
	case string:
		reader = bytes.NewBufferString(val)
	default:
		data, err := json.Marshal(val)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
		reader = bytes.NewBuffer(data)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if len(pathVars) > 0 {
		req = pathvar.WithVars(req, pathVars)
	}
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

func decodeJSON[T any](t *testing.T, body io.Reader) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(body).Decode(&out); err != nil {
		t.Fatalf("decode json failed: %v", err)
	}
	return out
}
