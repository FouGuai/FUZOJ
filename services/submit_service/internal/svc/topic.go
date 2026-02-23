package svc

import "github.com/zeromicro/go-queue/kq"

// TopicConfig defines routing topics for judge tasks.
type TopicConfig struct {
	Level0 string
	Level1 string
	Level2 string
	Level3 string
}

// TopicPushers holds Kafka pushers for each topic.
type TopicPushers struct {
	Level0 *kq.Pusher
	Level1 *kq.Pusher
	Level2 *kq.Pusher
	Level3 *kq.Pusher
}
