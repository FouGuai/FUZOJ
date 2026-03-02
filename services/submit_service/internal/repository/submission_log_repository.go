package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"fuzoj/internal/common/storage"
	appErr "fuzoj/pkg/errors"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	logKeyPrefixDefault  = "submission-logs"
	logCachePrefix       = "jl:"
	logMetaPrefix        = "meta:"
	defaultLogCacheTTL   = 10 * time.Minute
	defaultLogMaxInline  = 64 * 1024
	logTableName         = "submission_logs"
	logContentType       = "text/plain; charset=utf-8"
	logTypeShortCompile  = "c"
	logTypeShortError    = "e"
	logTypeShortRuntime  = "r"
	logTypeShortChecker  = "k"
	logTypeShortStdout   = "o"
	logTypeShortStderr   = "d"
	logTestIDPlaceholder = "0"
)

type SubmissionLogRepository struct {
	conn           sqlx.SqlConn
	redis          *redis.Redis
	storage        storage.ObjectStorage
	bucket         string
	keyPrefix      string
	maxInlineBytes int
	cacheTTL       time.Duration
}

type SubmissionLog struct {
	SubmissionID string
	LogType      string
	TestID       string
	Content      string
	LogPath      string
	LogSize      int64
	Truncated    bool
	UpdatedAt    time.Time
}

type logMeta struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Truncated bool   `json:"truncated"`
}

func NewSubmissionLogRepository(conn sqlx.SqlConn, redisClient *redis.Redis, storageClient storage.ObjectStorage, bucket, keyPrefix string, maxInlineBytes int, cacheTTL time.Duration) *SubmissionLogRepository {
	if keyPrefix == "" {
		keyPrefix = logKeyPrefixDefault
	}
	if maxInlineBytes <= 0 {
		maxInlineBytes = defaultLogMaxInline
	}
	if cacheTTL <= 0 {
		cacheTTL = defaultLogCacheTTL
	}
	return &SubmissionLogRepository{
		conn:           conn,
		redis:          redisClient,
		storage:        storageClient,
		bucket:         bucket,
		keyPrefix:      keyPrefix,
		maxInlineBytes: maxInlineBytes,
		cacheTTL:       cacheTTL,
	}
}

func (r *SubmissionLogRepository) SaveBatch(ctx context.Context, logs []LogRecord) error {
	for _, record := range logs {
		if err := r.Save(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (r *SubmissionLogRepository) Save(ctx context.Context, record LogRecord) error {
	logger := logx.WithContext(ctx)
	if record.SubmissionID == "" || record.LogType == "" {
		return appErr.ValidationError("submission_log", "invalid")
	}
	content := record.Content
	sizeBytes := int64(len([]byte(content)))
	logPath := ""
	truncated := false

	if sizeBytes > int64(r.maxInlineBytes) {
		if r.storage == nil || strings.TrimSpace(r.bucket) == "" {
			logger.Error("storage is required for large logs")
			return appErr.New(appErr.ServiceUnavailable).WithMessage("log storage is not configured")
		}
		objectKey := r.buildObjectKey(record.SubmissionID, record.LogType, record.TestID)
		reader := io.NopCloser(strings.NewReader(content))
		defer reader.Close()
		if err := r.storage.PutObject(ctx, r.bucket, objectKey, reader, sizeBytes, logContentType); err != nil {
			logger.Errorf("upload log failed: %v", err)
			return appErr.Wrapf(err, appErr.ServiceUnavailable, "upload log failed")
		}
		logPath = objectKey
		content = ""
	}

	if err := r.upsert(ctx, record.SubmissionID, record.LogType, record.TestID, content, logPath, sizeBytes, truncated); err != nil {
		logger.Errorf("store log failed: %v", err)
		return appErr.Wrapf(err, appErr.DatabaseError, "store log failed")
	}

	if r.redis != nil {
		if logPath != "" {
			meta := logMeta{Path: logPath, SizeBytes: sizeBytes, Truncated: truncated}
			payload, err := json.Marshal(meta)
			if err == nil {
				_ = r.redis.SetexCtx(ctx, r.cacheKey(record.SubmissionID, record.LogType, record.TestID), logMetaPrefix+string(payload), ttlSeconds(r.cacheTTL))
			}
		} else {
			_ = r.redis.SetexCtx(ctx, r.cacheKey(record.SubmissionID, record.LogType, record.TestID), content, ttlSeconds(r.cacheTTL))
		}
	}
	return nil
}

func (r *SubmissionLogRepository) Get(ctx context.Context, submissionID, logType, testID string) (SubmissionLog, error) {
	//logger := logx.WithContext(ctx)
	if submissionID == "" || logType == "" {
		return SubmissionLog{}, appErr.ValidationError("submission_log", "invalid")
	}
	if r.redis != nil {
		val, err := r.redis.GetCtx(ctx, r.cacheKey(submissionID, logType, testID))
		if err == nil && val != "" {
			if strings.HasPrefix(val, logMetaPrefix) {
				var meta logMeta
				if err := json.Unmarshal([]byte(strings.TrimPrefix(val, logMetaPrefix)), &meta); err == nil {
					content, err := r.readObject(ctx, meta.Path)
					if err != nil {
						return SubmissionLog{}, err
					}
					return SubmissionLog{
						SubmissionID: submissionID,
						LogType:      logType,
						TestID:       testID,
						Content:      content,
						LogPath:      meta.Path,
						LogSize:      meta.SizeBytes,
						Truncated:    meta.Truncated,
					}, nil
				}
			}
			return SubmissionLog{
				SubmissionID: submissionID,
				LogType:      logType,
				TestID:       testID,
				Content:      val,
				LogSize:      int64(len([]byte(val))),
			}, nil
		}
	}

	logItem, err := r.findOne(ctx, submissionID, logType, testID)
	if err != nil {
		return SubmissionLog{}, err
	}
	if logItem.LogPath != "" {
		content, err := r.readObject(ctx, logItem.LogPath)
		if err != nil {
			return SubmissionLog{}, err
		}
		logItem.Content = content
	}
	r.cache(ctx, logItem)
	return logItem, nil
}

func (r *SubmissionLogRepository) cache(ctx context.Context, logItem SubmissionLog) {
	if r.redis == nil {
		return
	}
	if logItem.LogPath != "" {
		meta := logMeta{Path: logItem.LogPath, SizeBytes: logItem.LogSize, Truncated: logItem.Truncated}
		if payload, err := json.Marshal(meta); err == nil {
			_ = r.redis.SetexCtx(ctx, r.cacheKey(logItem.SubmissionID, logItem.LogType, logItem.TestID), logMetaPrefix+string(payload), ttlSeconds(r.cacheTTL))
		}
		return
	}
	if logItem.Content != "" {
		_ = r.redis.SetexCtx(ctx, r.cacheKey(logItem.SubmissionID, logItem.LogType, logItem.TestID), logItem.Content, ttlSeconds(r.cacheTTL))
	}
}

func (r *SubmissionLogRepository) buildObjectKey(submissionID, logType, testID string) string {
	segments := []string{strings.TrimSuffix(r.keyPrefix, "/"), submissionID, logType}
	if testID != "" {
		segments = append(segments, testID)
	}
	return strings.Join(segments, "/") + ".log"
}

func (r *SubmissionLogRepository) readObject(ctx context.Context, objectKey string) (string, error) {
	if r.storage == nil {
		return "", appErr.New(appErr.ServiceUnavailable).WithMessage("log storage is not configured")
	}
	reader, err := r.storage.GetObject(ctx, r.bucket, objectKey)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.ServiceUnavailable, "read log failed")
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", appErr.Wrapf(err, appErr.ServiceUnavailable, "read log failed")
	}
	return string(data), nil
}

func (r *SubmissionLogRepository) cacheKey(submissionID, logType, testID string) string {
	shortType := logTypeShort(logType)
	shortTestID := logTestIDPlaceholder
	if testID != "" {
		shortTestID = hashKey(testID)
	}
	return fmt.Sprintf("%s%s:%s:%s", logCachePrefix, hashKey(submissionID), shortType, shortTestID)
}

func logTypeShort(logType string) string {
	switch logType {
	case LogTypeCompileLog:
		return logTypeShortCompile
	case LogTypeCompileError:
		return logTypeShortError
	case LogTypeRuntime:
		return logTypeShortRuntime
	case LogTypeChecker:
		return logTypeShortChecker
	case LogTypeStdout:
		return logTypeShortStdout
	case LogTypeStderr:
		return logTypeShortStderr
	default:
		return "x"
	}
}

func (r *SubmissionLogRepository) upsert(ctx context.Context, submissionID, logType, testID, content, logPath string, logSize int64, truncated bool) error {
	query := fmt.Sprintf(`insert into %s (submission_id, log_type, test_id, content, log_path, log_size, truncated, created_at, updated_at)
values (?, ?, ?, ?, ?, ?, ?, now(), now())
on duplicate key update content = values(content), log_path = values(log_path), log_size = values(log_size), truncated = values(truncated), updated_at = now()`, logTableName)
	_, err := r.conn.ExecCtx(ctx, query, submissionID, logType, testID, content, logPath, logSize, truncated)
	return err
}

func (r *SubmissionLogRepository) findOne(ctx context.Context, submissionID, logType, testID string) (SubmissionLog, error) {
	query := fmt.Sprintf("select submission_id, log_type, test_id, content, log_path, log_size, truncated, updated_at from %s where submission_id = ? and log_type = ? and test_id = ? limit 1", logTableName)
	var resp SubmissionLog
	if err := r.conn.QueryRowCtx(ctx, &resp, query, submissionID, logType, testID); err != nil {
		return SubmissionLog{}, appErr.Wrapf(err, appErr.DatabaseError, "log not found")
	}
	return resp, nil
}
