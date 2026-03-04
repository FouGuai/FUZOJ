package consumer

import (
	"fuzoj/services/contest_service/internal/config"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/service"
)

// BuildJudgeFinalKqConf builds kq config for judge final consumer.
func BuildJudgeFinalKqConf(c config.Config) kq.KqConf {
	consumers := c.JudgeFinal.PrefetchCount
	if consumers <= 0 {
		consumers = 1
	}
	processors := c.JudgeFinal.Concurrency
	if processors <= 0 {
		processors = 1
	}
	group := c.JudgeFinal.ConsumerGroup
	if group == "" {
		group = "contest-judge-final"
	}

	conf := kq.KqConf{
		ServiceConf: service.ServiceConf{
			Name: "contest-judge-final-consumer",
		},
		Brokers:    c.Kafka.Brokers,
		Group:      group,
		Topic:      c.JudgeFinal.Topic,
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
