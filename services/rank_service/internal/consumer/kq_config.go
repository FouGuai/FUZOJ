package consumer

import (
	"fuzoj/services/rank_service/internal/config"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/service"
)

// BuildRankUpdateKqConf builds kq config for rank update consumer.
func BuildRankUpdateKqConf(c config.Config) kq.KqConf {
	consumers := c.Rank.PrefetchCount
	if consumers <= 0 {
		consumers = 1
	}
	processors := c.Rank.Concurrency
	if processors <= 0 {
		processors = 1
	}
	group := c.Rank.ConsumerGroup
	if group == "" {
		group = "rank-service"
	}

	conf := kq.KqConf{
		ServiceConf: service.ServiceConf{
			Name: "rank-update-consumer",
		},
		Brokers:    c.Kafka.Brokers,
		Group:      group,
		Topic:      c.Rank.UpdateTopic,
		Consumers:  consumers,
		Processors: processors,
		MinBytes:   c.Kafka.MinBytes,
		MaxBytes:   c.Kafka.MaxBytes,
	}
	if conf.MinBytes <= 0 {
		conf.MinBytes = 10 * 1024
	}
	if conf.MaxBytes <= 0 {
		conf.MaxBytes = 10 * 1024 * 1024
	}
	return conf
}
