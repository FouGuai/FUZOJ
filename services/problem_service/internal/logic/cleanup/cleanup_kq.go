package cleanup

import (
	"fuzoj/services/problem_service/internal/config"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/service"
)

// BuildCleanupKqConf builds kq config for cleanup consumer.
func BuildCleanupKqConf(c config.Config) kq.KqConf {
	consumers := c.Cleanup.PrefetchCount
	if consumers <= 0 {
		consumers = 1
	}
	processors := c.Cleanup.Concurrency
	if processors <= 0 {
		processors = 1
	}
	group := c.Cleanup.ConsumerGroup
	if group == "" {
		group = "problem-cleanup"
	}

	conf := kq.KqConf{
		ServiceConf: service.ServiceConf{
			Name: "problem-cleanup-consumer",
		},
		Brokers:    c.Kafka.Brokers,
		Group:      group,
		Topic:      c.Cleanup.Topic,
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
