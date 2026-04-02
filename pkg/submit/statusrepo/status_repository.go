package statusrepo

import (
	"context"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/pkg/submit/statuscache"
	"fuzoj/pkg/submit/statusflow"
	"fuzoj/pkg/submit/statusutil"
	"fuzoj/pkg/submit/statuswriter"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	defaultStatusCacheTTL      = 30 * time.Minute
	defaultStatusCacheEmptyTTL = 5 * time.Minute
)

type StatusRepositoryConfig[T any] struct {
	Cache    *redis.Redis
	PubSub   *red.Client
	TTL      time.Duration
	EmptyTTL time.Duration

	GetSubmissionID func(T) string
	GetStatusLabel  func(T) string
	Encode          func(T) (string, error)
	Decode          func(string) (T, error)
	BuildUnknown    func(string) T

	LoadOneFromDB   func(context.Context, string) (T, bool, error)
	LoadBatchFromDB func(context.Context, []string) (map[string]T, error)
	LoadFinalFromDB func(context.Context, string) (T, bool, error)

	ToWriterPayload func(T) (statuswriter.StatusPayload, error)
	IsFinalStatus   func(T) bool
	OnFinalStatus   func(context.Context, T) error
	PersistFinal    func(context.Context, T) error

	CheckOwner func(context.Context, string, int64) error
}

// StatusRepository is a single reusable cache-aside status repository implementation.
type StatusRepository[T any] struct {
	cache    *redis.Redis
	pubsub   *red.Client
	updater  *statusflow.Updater
	ttl      time.Duration
	emptyTTL time.Duration

	getSubmissionID func(T) string
	getStatusLabel  func(T) string
	encode          func(T) (string, error)
	decode          func(string) (T, error)
	buildUnknown    func(string) T

	loadOneFromDB   func(context.Context, string) (T, bool, error)
	loadBatchFromDB func(context.Context, []string) (map[string]T, error)
	loadFinalFromDB func(context.Context, string) (T, bool, error)

	toWriterPayload func(T) (statuswriter.StatusPayload, error)
	isFinalStatus   func(T) bool
	onFinalStatus   func(context.Context, T) error
	persistFinal    func(context.Context, T) error

	checkOwner func(context.Context, string, int64) error
}

func NewStatusRepository[T any](cfg StatusRepositoryConfig[T]) *StatusRepository[T] {
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultStatusCacheTTL
	}
	emptyTTL := cfg.EmptyTTL
	if emptyTTL <= 0 {
		emptyTTL = defaultStatusCacheEmptyTTL
	}

	return &StatusRepository[T]{
		cache:           cfg.Cache,
		pubsub:          cfg.PubSub,
		updater:         statusflow.NewUpdater(cfg.Cache, cfg.PubSub, ttl),
		ttl:             ttl,
		emptyTTL:        emptyTTL,
		getSubmissionID: cfg.GetSubmissionID,
		getStatusLabel:  cfg.GetStatusLabel,
		encode:          cfg.Encode,
		decode:          cfg.Decode,
		buildUnknown:    cfg.BuildUnknown,
		loadOneFromDB:   cfg.LoadOneFromDB,
		loadBatchFromDB: cfg.LoadBatchFromDB,
		loadFinalFromDB: cfg.LoadFinalFromDB,
		toWriterPayload: cfg.ToWriterPayload,
		isFinalStatus:   cfg.IsFinalStatus,
		onFinalStatus:   cfg.OnFinalStatus,
		persistFinal:    cfg.PersistFinal,
		checkOwner:      cfg.CheckOwner,
	}
}

func (r *StatusRepository[T]) SetStatusPubSub(client *red.Client) {
	if r == nil {
		return
	}
	r.pubsub = client
	r.updater = statusflow.NewUpdater(r.cache, client, r.ttl)
}

func (r *StatusRepository[T]) Get(ctx context.Context, submissionID string) (T, error) {
	var zero T
	if submissionID == "" {
		return zero, appErr.ValidationError("submission_id", "required")
	}
	if r == nil || r.loadOneFromDB == nil || r.decode == nil || r.encode == nil || r.buildUnknown == nil {
		return zero, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}

	if r.cache != nil {
		cached, hit, err := statuscache.Get(ctx, r.cache, submissionID)
		if err != nil {
			return zero, err
		}
		if hit {
			if cached == statuscache.NullValue {
				return r.buildUnknown(submissionID), nil
			}
			status, err := r.decode(cached)
			if err == nil {
				return status, nil
			}
		}
	}

	status, found, err := r.loadOneFromDB(ctx, submissionID)
	if err != nil {
		return zero, err
	}
	if !found {
		unknown := r.buildUnknown(submissionID)
		r.cacheStatus(ctx, unknown, r.emptyTTL)
		return unknown, nil
	}
	r.cacheStatus(ctx, status, r.ttl)
	return status, nil
}

func (r *StatusRepository[T]) GetBatch(ctx context.Context, submissionIDs []string) ([]T, []string, error) {
	if len(submissionIDs) == 0 {
		return nil, nil, appErr.ValidationError("submission_ids", "required")
	}
	for _, id := range submissionIDs {
		if id == "" {
			return nil, nil, appErr.ValidationError("submission_id", "required")
		}
	}
	if r == nil || r.loadBatchFromDB == nil || r.decode == nil || r.encode == nil || r.buildUnknown == nil {
		return nil, nil, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}

	statusByID := make(map[string]T, len(submissionIDs))
	missingIDs := make([]string, 0, len(submissionIDs))
	missingSet := make(map[string]struct{}, len(submissionIDs))

	if r.cache != nil {
		keys := make([]string, 0, len(submissionIDs))
		for _, id := range submissionIDs {
			keys = append(keys, statuscache.PrimaryKey(id))
		}
		values, err := r.cache.MgetCtx(ctx, keys...)
		if err != nil {
			return nil, nil, err
		}
		for i, raw := range values {
			if i >= len(submissionIDs) {
				break
			}
			id := submissionIDs[i]
			if raw == "" {
				missingIDs = append(missingIDs, id)
				missingSet[id] = struct{}{}
				continue
			}
			if raw == statuscache.NullValue {
				statusByID[id] = r.buildUnknown(id)
				continue
			}
			decoded, err := r.decode(raw)
			if err != nil {
				if _, ok := missingSet[id]; !ok {
					missingIDs = append(missingIDs, id)
					missingSet[id] = struct{}{}
				}
				continue
			}
			statusByID[id] = decoded
		}
		if len(values) < len(submissionIDs) {
			for _, id := range submissionIDs[len(values):] {
				if _, ok := missingSet[id]; ok {
					continue
				}
				missingIDs = append(missingIDs, id)
				missingSet[id] = struct{}{}
			}
		}
	} else {
		missingIDs = append(missingIDs, submissionIDs...)
	}

	if len(missingIDs) > 0 {
		dbFound, err := r.loadBatchFromDB(ctx, missingIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, id := range missingIDs {
			if status, ok := dbFound[id]; ok {
				statusByID[id] = status
				r.cacheStatus(ctx, status, r.ttl)
				continue
			}
			unknown := r.buildUnknown(id)
			statusByID[id] = unknown
			r.cacheStatus(ctx, unknown, r.emptyTTL)
		}
	}

	statuses := make([]T, 0, len(submissionIDs))
	for _, id := range submissionIDs {
		if status, ok := statusByID[id]; ok {
			statuses = append(statuses, status)
		}
	}
	return statuses, nil, nil
}

func (r *StatusRepository[T]) GetFinalDetail(ctx context.Context, submissionID string) (T, error) {
	var zero T
	if submissionID == "" {
		return zero, appErr.ValidationError("submission_id", "required")
	}
	if r == nil {
		return zero, appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}
	if r.loadFinalFromDB == nil {
		return r.Get(ctx, submissionID)
	}
	status, found, err := r.loadFinalFromDB(ctx, submissionID)
	if err != nil {
		return zero, err
	}
	if !found {
		return r.buildUnknown(submissionID), nil
	}
	return status, nil
}

func (r *StatusRepository[T]) Save(ctx context.Context, status T) error {
	if r == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("status repository is not configured")
	}
	submissionID := ""
	if r.getSubmissionID != nil {
		submissionID = r.getSubmissionID(status)
	}
	if submissionID == "" {
		return appErr.ValidationError("submission_id", "required")
	}
	if r.isFinalStatus != nil && r.isFinalStatus(status) && r.onFinalStatus != nil {
		if err := r.onFinalStatus(ctx, status); err != nil {
			logx.WithContext(ctx).Errorf("enqueue final status failed: %v", err)
		}
	}
	if r.cache == nil || r.toWriterPayload == nil {
		return nil
	}
	if r.updater == nil {
		r.updater = statusflow.NewUpdater(r.cache, r.pubsub, r.ttl)
	}
	payload, err := r.toWriterPayload(status)
	if err != nil {
		return err
	}
	accept, reason, err := r.updater.ApplySummary(ctx, payload)
	if err != nil {
		return err
	}
	if !accept {
		label := ""
		if r.getStatusLabel != nil {
			label = r.getStatusLabel(status)
		}
		logx.WithContext(ctx).Infof("skip non-monotonic status update submission_id=%s status=%s reason=%s", submissionID, label, reason)
	}
	return nil
}

func (r *StatusRepository[T]) PersistFinalStatus(ctx context.Context, status T) error {
	if r == nil || r.persistFinal == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("persist final status is not configured")
	}
	return r.persistFinal(ctx, status)
}

func (r *StatusRepository[T]) CheckSubmissionOwner(ctx context.Context, submissionID string, userID int64) error {
	if r == nil || r.checkOwner == nil {
		return appErr.New(appErr.ServiceUnavailable).WithMessage("owner checker is not configured")
	}
	return r.checkOwner(ctx, submissionID, userID)
}

func (r *StatusRepository[T]) GetLatestStatus(ctx context.Context, submissionID string) (T, error) {
	return r.Get(ctx, submissionID)
}

func (r *StatusRepository[T]) GetFinalStatus(ctx context.Context, submissionID string) (T, error) {
	return r.GetFinalDetail(ctx, submissionID)
}

func (r *StatusRepository[T]) cacheStatus(ctx context.Context, status T, ttl time.Duration) {
	if r == nil || r.cache == nil || r.encode == nil {
		return
	}
	submissionID := ""
	if r.getSubmissionID != nil {
		submissionID = r.getSubmissionID(status)
	}
	if submissionID == "" {
		return
	}
	payload, err := r.encode(status)
	if err != nil || payload == "" {
		return
	}
	_ = statuscache.Set(ctx, r.cache, submissionID, payload, statusutil.TTLSeconds(ttl))
}
