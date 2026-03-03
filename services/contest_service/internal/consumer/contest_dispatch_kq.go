package consumer

import (
	"fuzoj/services/contest_service/internal/config"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/service"
)

// BuildContestDispatchKqConf builds kq config for contest dispatch consumer.
func BuildContestDispatchKqConf(c config.Config) kq.KqConf {
	consumers := c.ContestDispatch.PrefetchCount
	if consumers <= 0 {
		consumers = 1
	}
	processors := c.ContestDispatch.Concurrency
	if processors <= 0 {
		processors = 1
	}
	group := c.ContestDispatch.ConsumerGroup
	if group == "" {
		group = "contest-dispatch"
	}

	conf := kq.KqConf{
		ServiceConf: service.ServiceConf{
			Name: "contest-dispatch-consumer",
		},
		Brokers:    c.Kafka.Brokers,
		Group:      group,
		Topic:      c.ContestDispatch.Topic,
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
