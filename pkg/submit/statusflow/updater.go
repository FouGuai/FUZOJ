package statusflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"fuzoj/pkg/submit/statuscache"
	"fuzoj/pkg/submit/statusmonotonic"
	"fuzoj/pkg/submit/statuspubsub"
	"fuzoj/pkg/submit/statusutil"
	"fuzoj/pkg/submit/statuswriter"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// Updater applies monotonic status updates into cache and publishes status updates.
type Updater struct {
	cache  *redis.Redis
	pubsub *red.Client
	ttl    time.Duration
}

// NewUpdater creates a status updater.
func NewUpdater(cache *redis.Redis, pubsub *red.Client, ttl time.Duration) *Updater {
	return &Updater{
		cache:  cache,
		pubsub: pubsub,
		ttl:    ttl,
	}
}

// ApplySummary applies summary status with monotonic checks.
func (u *Updater) ApplySummary(ctx context.Context, next statuswriter.StatusPayload) (bool, string, error) {
	if next.SubmissionID == "" {
		return false, "", fmt.Errorf("submission_id is required")
	}
	if u.cache != nil {
		cached, hit, err := statuscache.Get(ctx, u.cache, next.SubmissionID)
		if err != nil {
			return false, "", err
		}
		if hit && cached != "" && cached != statuscache.NullValue {
			var current statuswriter.StatusPayload
			if err := json.Unmarshal([]byte(cached), &current); err == nil {
				accept, reason := statusmonotonic.ShouldAccept(
					current.Status,
					current.Progress.DoneTests,
					current.Progress.TotalTests,
					next.Status,
					next.Progress.DoneTests,
					next.Progress.TotalTests,
				)
				if !accept {
					return false, reason, nil
				}
			}
		}
		summary := statuswriter.BuildSummary(next)
		payload, err := json.Marshal(summary)
		if err != nil {
			return false, "", fmt.Errorf("marshal status failed: %w", err)
		}
		if err := statuscache.Set(ctx, u.cache, next.SubmissionID, string(payload), statusutil.TTLSeconds(u.ttl)); err != nil {
			return false, "", err
		}
	}
	if err := statuspubsub.Publish(ctx, u.pubsub, next.SubmissionID); err != nil {
		return false, "", err
	}
	return true, "", nil
}
