package metapubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const channelName = "problem:meta:pubsub"

// Event describes a latest-problem-meta invalidation broadcast.
type Event struct {
	ProblemID int64 `json:"problem_id"`
	Version   int32 `json:"version"`
	UpdatedAt int64 `json:"updated_at"`
}

func NewClient(conf redis.RedisConf) *red.Client {
	if strings.TrimSpace(conf.Host) == "" {
		return nil
	}
	if conf.Type != "" && conf.Type != "node" {
		return nil
	}
	return red.NewClient(&red.Options{
		Addr:     conf.Host,
		Username: conf.User,
		Password: conf.Pass,
	})
}

func Channel() string {
	return channelName
}

func PublishInvalidation(ctx context.Context, client *red.Client, problemID int64, version int32) error {
	if problemID <= 0 {
		return fmt.Errorf("problem_id is required")
	}
	if client == nil {
		return nil
	}
	event := Event{
		ProblemID: problemID,
		Version:   version,
		UpdatedAt: time.Now().UnixMilli(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal problem meta pubsub event failed: %w", err)
	}
	if err := client.Publish(ctx, Channel(), string(payload)).Err(); err != nil {
		return fmt.Errorf("publish problem meta pubsub event failed: %w", err)
	}
	return nil
}
