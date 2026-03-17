package statuspubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	red "github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const channelPrefix = "submission:status:pubsub:"

type Event struct {
	SubmissionID string `json:"submission_id"`
	UpdatedAt    int64  `json:"updated_at"`
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

func Channel(submissionID string) string {
	return channelPrefix + strings.TrimSpace(submissionID)
}

func Publish(ctx context.Context, client *red.Client, submissionID string) error {
	id := strings.TrimSpace(submissionID)
	if id == "" {
		return fmt.Errorf("submission_id is required")
	}
	if client == nil {
		return nil
	}
	event := Event{SubmissionID: id, UpdatedAt: time.Now().UnixMilli()}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal status pubsub event failed: %w", err)
	}
	if err := client.Publish(ctx, Channel(id), string(payload)).Err(); err != nil {
		return fmt.Errorf("publish status pubsub event failed: %w", err)
	}
	return nil
}
