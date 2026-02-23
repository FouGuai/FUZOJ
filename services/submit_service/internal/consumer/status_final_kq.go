package consumer

import (
	"fuzoj/services/submit_service/internal/config"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/service"
)

// BuildStatusFinalKqConf builds kq config for status final consumer.
func BuildStatusFinalKqConf(c config.Config) kq.KqConf {
	consumers := c.Submit.StatusFinalConsumer.PrefetchCount
	if consumers <= 0 {
		consumers = 1
	}
	processors := c.Submit.StatusFinalConsumer.Concurrency
	if processors <= 0 {
		processors = 1
	}
	group := c.Submit.StatusFinalConsumer.ConsumerGroup
	if group == "" {
		group = "submit-service-status-final"
	}

	conf := kq.KqConf{
		ServiceConf: service.ServiceConf{
			Name: "submit-status-final-consumer",
		},
		Brokers:    c.Kafka.Brokers,
		Group:      group,
		Topic:      c.Submit.StatusFinalTopic,
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
