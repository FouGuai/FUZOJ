package mq

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"fuzoj/internal/common/mq/weighted_kq"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/queue"
)

// KafkaConfig defines configuration for Kafka adapter.
type KafkaConfig struct {
	Brokers  []string
	ClientID string

	// Producer settings
	RequiredAcks int
	BatchSize    int
	BatchTimeout time.Duration
	Compression  string

	// Consumer settings
	MinBytes int
	MaxBytes int
	MaxWait  time.Duration

	// Dialer settings
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// WeightedTopic defines a topic with fetch weight.
type WeightedTopic struct {
	Topic  string
	Weight int
}

type kafkaSubscription struct {
	topic   string
	weights []WeightedTopic
	handler HandlerFunc
	opts    SubscribeOptions
	limiter FetchLimiter
	queue   queue.MessageQueue
}

// KafkaQueue implements MessageQueue with go-zero kq.
type KafkaQueue struct {
	config KafkaConfig

	mu            sync.Mutex
	subscriptions []*kafkaSubscription
	pushers       map[string]*kq.Pusher
	started       bool
	closed        bool
	paused        atomic.Bool
}

// NewKafkaQueue creates a Kafka-backed message queue adapter.
func NewKafkaQueue(cfg KafkaConfig) (*KafkaQueue, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("brokers are required")
	}
	return &KafkaQueue{
		config:  cfg,
		pushers: make(map[string]*kq.Pusher),
	}, nil
}

// Publish publishes a message to a topic.
func (k *KafkaQueue) Publish(ctx context.Context, topic string, message *Message) error {
	if message == nil {
		return errors.New("message is nil")
	}
	if topic == "" {
		return errors.New("topic is required")
	}
	pusher := k.getPusher(topic)
	return pusher.PushWithKey(ctx, message.ID, string(message.Body))
}

// PublishBatch publishes multiple messages in a batch.
func (k *KafkaQueue) PublishBatch(ctx context.Context, topic string, messages []*Message) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	if len(messages) == 0 {
		return errors.New("messages are required")
	}
	pusher := k.getPusher(topic)
	for _, msg := range messages {
		if msg == nil {
			return errors.New("message is nil")
		}
		if err := pusher.PushWithKey(ctx, msg.ID, string(msg.Body)); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe subscribes to a topic with default options.
func (k *KafkaQueue) Subscribe(ctx context.Context, topic string, handler HandlerFunc) error {
	return k.SubscribeWithOptions(ctx, topic, handler, nil)
}

// SubscribeWithOptions subscribes to a topic with custom options.
func (k *KafkaQueue) SubscribeWithOptions(ctx context.Context, topic string, handler HandlerFunc, opts *SubscribeOptions) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}
	var options SubscribeOptions
	if opts != nil {
		options = *opts
	}
	options.SetDefaults()
	if options.ConsumerGroup == "" {
		options.ConsumerGroup = "fuzoj-legacy"
	}

	sub := &kafkaSubscription{
		topic:   topic,
		handler: handler,
		opts:    options,
	}
	return k.addSubscription(sub)
}

// SubscribeWeighted subscribes to multiple topics with weights and a fetch limiter.
func (k *KafkaQueue) SubscribeWeighted(ctx context.Context, topics []WeightedTopic, handler HandlerFunc, opts *SubscribeOptions, limiter FetchLimiter) error {
	if len(topics) == 0 {
		return errors.New("topics are required")
	}
	if handler == nil {
		return errors.New("handler is required")
	}
	var options SubscribeOptions
	if opts != nil {
		options = *opts
	}
	options.SetDefaults()
	if options.ConsumerGroup == "" {
		options.ConsumerGroup = "fuzoj-legacy"
	}
	sub := &kafkaSubscription{
		weights: topics,
		handler: handler,
		opts:    options,
		limiter: limiter,
	}
	return k.addSubscription(sub)
}

// Start starts consuming messages for all subscriptions.
func (k *KafkaQueue) Start() error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return errors.New("message queue is closed")
	}
	if k.started {
		k.mu.Unlock()
		return nil
	}
	subs := append([]*kafkaSubscription(nil), k.subscriptions...)
	k.started = true
	k.mu.Unlock()

	for _, sub := range subs {
		queue, err := k.buildQueue(sub)
		if err != nil {
			return err
		}
		sub.queue = queue
		go queue.Start()
	}
	return nil
}

// Stop stops all consumers.
func (k *KafkaQueue) Stop() error {
	k.mu.Lock()
	subs := append([]*kafkaSubscription(nil), k.subscriptions...)
	k.mu.Unlock()
	for _, sub := range subs {
		if sub.queue != nil {
			sub.queue.Stop()
		}
	}
	return nil
}

// Pause pauses consumption.
func (k *KafkaQueue) Pause() error {
	k.paused.Store(true)
	return nil
}

// Resume resumes consumption after pause.
func (k *KafkaQueue) Resume() error {
	k.paused.Store(false)
	return nil
}

// Ping verifies the Kafka connection.
func (k *KafkaQueue) Ping(ctx context.Context) error {
	return nil
}

// Close closes the producer and stops consumers.
func (k *KafkaQueue) Close() error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return nil
	}
	k.closed = true
	k.mu.Unlock()

	_ = k.Stop()
	for _, p := range k.pushers {
		_ = p.Close()
	}
	return nil
}

func (k *KafkaQueue) addSubscription(sub *kafkaSubscription) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return errors.New("message queue is closed")
	}
	k.subscriptions = append(k.subscriptions, sub)
	if k.started {
		queue, err := k.buildQueue(sub)
		if err != nil {
			return err
		}
		sub.queue = queue
		go queue.Start()
	}
	return nil
}

func (k *KafkaQueue) buildQueue(sub *kafkaSubscription) (queue.MessageQueue, error) {
	handler := k.buildHandler(sub)
	if len(sub.weights) > 0 {
		topics := make([]string, 0, len(sub.weights))
		weights := make(map[string]int, len(sub.weights))
		for _, t := range sub.weights {
			if t.Topic == "" {
				continue
			}
			topics = append(topics, t.Topic)
			weights[t.Topic] = t.Weight
		}
		confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
			Brokers:         k.config.Brokers,
			Group:           sub.opts.ConsumerGroup,
			Topics:          topics,
			TopicWeights:    weights,
			ConsumersTotal:  sub.opts.PrefetchCount,
			ProcessorsTotal: sub.opts.Concurrency,
			MinBytes:        k.config.MinBytes,
			MaxBytes:        k.config.MaxBytes,
			ServiceName:     k.config.ClientID,
		})
		if err != nil {
			return nil, err
		}
		return weighted_kq.NewWeightedKqQueues(confs, handler)
	}
	conf := kq.KqConf{
		Brokers:    k.config.Brokers,
		Group:      sub.opts.ConsumerGroup,
		Topic:      sub.topic,
		Consumers:  sub.opts.PrefetchCount,
		Processors: sub.opts.Concurrency,
		MinBytes:   k.config.MinBytes,
		MaxBytes:   k.config.MaxBytes,
	}
	if k.config.ClientID != "" {
		conf.Name = k.config.ClientID + "-" + sub.topic
	} else {
		conf.Name = "kq-" + sub.topic
	}
	return kq.NewQueue(conf, handler)
}

func (k *KafkaQueue) buildHandler(sub *kafkaSubscription) kq.ConsumeHandler {
	return kq.WithHandle(func(ctx context.Context, key, value string) error {
		for k.paused.Load() {
			time.Sleep(100 * time.Millisecond)
		}
		if sub.limiter != nil {
			if err := sub.limiter.Acquire(ctx); err != nil {
				return err
			}
			defer sub.limiter.Release()
		}
		msg := &Message{
			ID:         key,
			Body:       []byte(value),
			Headers:    make(map[string]string),
			Timestamp:  time.Now(),
			MaxRetries: sub.opts.MaxRetries,
		}
		if sub.opts.MessageTTL > 0 && !msg.Timestamp.IsZero() {
			if time.Since(msg.Timestamp) > sub.opts.MessageTTL {
				return nil
			}
		}
		maxRetries := sub.opts.MaxRetries
		if maxRetries < 0 {
			maxRetries = 0
		}
		retryDelay := sub.opts.RetryDelay
		if retryDelay <= 0 {
			retryDelay = time.Second
		}
		for attempt := 0; attempt <= maxRetries; attempt++ {
			if err := sub.handler(ctx, msg); err == nil {
				return nil
			} else if attempt >= maxRetries {
				if sub.opts.DeadLetterTopic != "" {
					_ = k.Publish(ctx, sub.opts.DeadLetterTopic, msg)
				}
				return nil
			}
			time.Sleep(retryDelay)
		}
		return nil
	})
}

func (k *KafkaQueue) getPusher(topic string) *kq.Pusher {
	k.mu.Lock()
	defer k.mu.Unlock()
	if pusher, ok := k.pushers[topic]; ok {
		return pusher
	}
	pusher := kq.NewPusher(k.config.Brokers, topic, kq.WithSyncPush())
	k.pushers[topic] = pusher
	return pusher
}
