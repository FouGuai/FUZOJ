package weighted_kq

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

// WeightedKqOptions defines weighted Kafka queue settings.
type WeightedKqOptions struct {
	Brokers         []string
	Group           string
	Topics          []string
	TopicWeights    map[string]int
	ConsumersTotal  int
	ProcessorsTotal int
	MinBytes        int
	MaxBytes        int
	ServiceName     string
	RetryTopic      string
	AutoAddRetry    bool
}

// BuildWeightedKqConfs builds kq configs for topics with weights.
func BuildWeightedKqConfs(opts WeightedKqOptions) ([]kq.KqConf, error) {
	if len(opts.Brokers) == 0 {
		return nil, errors.New("brokers are required")
	}
	topics := append([]string(nil), opts.Topics...)
	if opts.AutoAddRetry && opts.RetryTopic != "" && !containsTopic(topics, opts.RetryTopic) {
		topics = append(topics, opts.RetryTopic)
	}
	if len(topics) == 0 {
		return nil, errors.New("topics are required")
	}
	weights := buildTopicWeights(topics, opts.TopicWeights)
	consumers := allocByWeight(topics, weights, opts.ConsumersTotal, "consumers")
	processors := allocByWeight(topics, weights, opts.ProcessorsTotal, "processors")

	confs := make([]kq.KqConf, 0, len(topics))
	for _, topic := range topics {
		conf := kq.KqConf{
			Brokers:    opts.Brokers,
			Group:      defaultGroup(opts.Group),
			Topic:      topic,
			Consumers:  consumers[topic],
			Processors: processors[topic],
			MinBytes:   opts.MinBytes,
			MaxBytes:   opts.MaxBytes,
		}
		if opts.ServiceName != "" {
			conf.Name = fmt.Sprintf("%s-%s", opts.ServiceName, topic)
		} else {
			conf.Name = fmt.Sprintf("kq-%s", topic)
		}
		confs = append(confs, conf)
	}
	return confs, nil
}

// NewWeightedKqQueues creates a group queue for multiple kq configs.
func NewWeightedKqQueues(confs []kq.KqConf, handler kq.ConsumeHandler, opts ...kq.QueueOption) (queue.MessageQueue, error) {
	if len(confs) == 0 {
		return nil, errors.New("kq configs are required")
	}
	if handler == nil {
		return nil, errors.New("consume handler is required")
	}
	queues := make([]queue.MessageQueue, 0, len(confs))
	for _, conf := range confs {
		q, err := kq.NewQueue(conf, handler, opts...)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}
	return &queueGroup{queues: queues}, nil
}

type queueGroup struct {
	queues []queue.MessageQueue
}

func (g *queueGroup) Start() {
	var wg sync.WaitGroup
	for _, q := range g.queues {
		wg.Add(1)
		go func(q queue.MessageQueue) {
			defer wg.Done()
			q.Start()
		}(q)
	}
	wg.Wait()
}

func (g *queueGroup) Stop() {
	for _, q := range g.queues {
		q.Stop()
	}
}

func buildTopicWeights(topics []string, explicit map[string]int) map[string]int {
	weights := make(map[string]int, len(topics))
	if len(explicit) == 0 {
		defaults := defaultWeights(len(topics))
		for i, topic := range topics {
			if topic == "" {
				continue
			}
			weights[topic] = defaults[i]
		}
		return weights
	}
	for _, topic := range topics {
		if topic == "" {
			continue
		}
		weight := explicit[topic]
		if weight <= 0 {
			weight = 1
		}
		weights[topic] = weight
	}
	return weights
}

func defaultWeights(count int) []int {
	base := []int{8, 4, 2, 1}
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		if i < len(base) {
			out = append(out, base[i])
			continue
		}
		out = append(out, 1)
	}
	return out
}

func allocByWeight(topics []string, weights map[string]int, total int, label string) map[string]int {
	if total <= 0 {
		total = 1
	}
	result := make(map[string]int, len(topics))
	weightTotal := 0
	for _, topic := range topics {
		weightTotal += maxInt(1, weights[topic])
	}
	if total < len(topics) {
		logx.Infof("weighted kq %s total=%d < topics=%d, using minimum 1 per topic", label, total, len(topics))
		for _, topic := range topics {
			result[topic] = 1
		}
		return result
	}

	type rem struct {
		topic    string
		fraction float64
	}
	remainders := make([]rem, 0, len(topics))
	assigned := 0
	for _, topic := range topics {
		weight := maxInt(1, weights[topic])
		raw := float64(total) * float64(weight) / float64(weightTotal)
		count := int(raw)
		if count < 1 {
			count = 1
		}
		result[topic] = count
		assigned += count
		remainders = append(remainders, rem{topic: topic, fraction: raw - float64(count)})
	}

	if assigned < total {
		sort.SliceStable(remainders, func(i, j int) bool {
			return remainders[i].fraction > remainders[j].fraction
		})
		for i := 0; i < total-assigned; i++ {
			idx := i % len(remainders)
			result[remainders[idx].topic]++
		}
	}
	return result
}

func containsTopic(topics []string, topic string) bool {
	for _, t := range topics {
		if t == topic {
			return true
		}
	}
	return false
}

func defaultGroup(group string) string {
	if group == "" {
		return "fuzoj-judge"
	}
	return group
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
