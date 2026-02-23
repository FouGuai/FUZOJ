package svc

import "context"

// TopicConfig defines routing topics for judge tasks.
type TopicConfig struct {
	Level0 string
	Level1 string
	Level2 string
	Level3 string
}

// TopicPusher defines minimal pusher interface for publishing judge tasks.
type TopicPusher interface {
	PushWithKey(ctx context.Context, key, value string) error
	Close() error
}

// TopicPushers holds Kafka pushers for each topic.
type TopicPushers struct {
	Level0 TopicPusher
	Level1 TopicPusher
	Level2 TopicPusher
	Level3 TopicPusher
}
