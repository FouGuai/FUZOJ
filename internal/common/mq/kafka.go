package mq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	headerID         = "x-message-id"
	headerTimestamp  = "x-message-ts"
	headerPriority   = "x-message-priority"
	headerRetryCount = "x-message-retry"
	headerMaxRetries = "x-message-max-retries"
	headerExpiration = "x-message-expiration-ms"
)

// KafkaConfig defines configuration for Kafka implementation.
type KafkaConfig struct {
	Brokers  []string
	ClientID string

	// Producer settings
	RequiredAcks kafka.RequiredAcks
	BatchSize    int
	BatchTimeout time.Duration
	Compression  kafka.Compression

	// Consumer settings
	MinBytes int
	MaxBytes int
	MaxWait  time.Duration

	// Dialer settings
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// KafkaQueue implements MessageQueue using Kafka.
type KafkaQueue struct {
	config KafkaConfig
	writer *kafka.Writer
	dialer *kafka.Dialer

	mu            sync.Mutex
	subscriptions []*kafkaSubscription
	started       bool
	closed        bool
	paused        atomic.Bool
}

type kafkaSubscription struct {
	topic   string
	handler HandlerFunc
	opts    SubscribeOptions
	baseCtx context.Context

	reader *kafka.Reader
	readers []*kafka.Reader
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	limiter FetchLimiter
}

// WeightedTopic defines a topic with fetch weight.
type WeightedTopic struct {
	Topic  string
	Weight int
}

// NewKafkaQueue creates a Kafka-backed message queue.
func NewKafkaQueue(cfg KafkaConfig) (*KafkaQueue, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("brokers are required")
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}
	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = 50 * time.Millisecond
	}
	if cfg.MinBytes == 0 {
		cfg.MinBytes = 1 << 10
	}
	if cfg.MaxBytes == 0 {
		cfg.MaxBytes = 10 << 20
	}
	if cfg.MaxWait == 0 {
		cfg.MaxWait = time.Second
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 10 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.RequiredAcks == 0 {
		cfg.RequiredAcks = kafka.RequireOne
	}

	dialer := &kafka.Dialer{
		ClientID:  cfg.ClientID,
		Timeout:   cfg.DialTimeout,
		DualStack: true,
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: cfg.RequiredAcks,
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		Compression:  cfg.Compression,
		Transport: &kafka.Transport{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, network, address)
			},
			ClientID: cfg.ClientID,
		},
	}

	return &KafkaQueue{
		config: cfg,
		writer: writer,
		dialer: dialer,
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
	msg := toKafkaMessage(topic, message)
	return k.writer.WriteMessages(ctx, msg)
}

// PublishBatch publishes multiple messages in a batch.
func (k *KafkaQueue) PublishBatch(ctx context.Context, topic string, messages []*Message) error {
	if topic == "" {
		return errors.New("topic is required")
	}
	if len(messages) == 0 {
		return errors.New("messages are required")
	}
	kmsgs := make([]kafka.Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			return errors.New("message is nil")
		}
		kmsgs = append(kmsgs, toKafkaMessage(topic, msg))
	}
	return k.writer.WriteMessages(ctx, kmsgs...)
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
		options.ConsumerGroup = fmt.Sprintf("fuzoj-%s", topic)
	}

	sub := &kafkaSubscription{
		topic:   topic,
		handler: handler,
		opts:    options,
		baseCtx: ctx,
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return errors.New("message queue is closed")
	}
	k.subscriptions = append(k.subscriptions, sub)
	if k.started {
		return k.startSubscription(sub)
	}
	return nil
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
		options.ConsumerGroup = "fuzoj-judge"
	}

	for _, t := range topics {
		if t.Topic == "" {
			return errors.New("topic is required")
		}
		if t.Weight <= 0 {
			return errors.New("topic weight must be positive")
		}
	}

	sub := &kafkaSubscription{
		handler: handler,
		opts:    options,
		baseCtx: ctx,
		limiter: limiter,
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return errors.New("message queue is closed")
	}
	k.subscriptions = append(k.subscriptions, sub)
	if k.started {
		return k.startWeightedSubscription(sub, topics)
	}
	sub.topic = "" // weighted mode
	sub.opts.QueueName = encodeWeightedTopics(topics)
	return nil
}

// Start starts consuming messages for all subscriptions.
func (k *KafkaQueue) Start() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return errors.New("message queue is closed")
	}
	if k.started {
		return nil
	}
	for _, sub := range k.subscriptions {
		if sub.opts.QueueName != "" && sub.topic == "" {
			topics := decodeWeightedTopics(sub.opts.QueueName)
			if err := k.startWeightedSubscription(sub, topics); err != nil {
				return err
			}
			continue
		}
		if err := k.startSubscription(sub); err != nil {
			return err
		}
	}
	k.started = true
	return nil
}

func (k *KafkaQueue) startWeightedSubscription(sub *kafkaSubscription, topics []WeightedTopic) error {
	readers := make([]*kafka.Reader, 0, len(topics))
	schedule := buildWeightedSchedule(topics)
	if len(schedule) == 0 {
		return errors.New("no weighted topics provided")
	}

	for _, t := range topics {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:     k.config.Brokers,
			Topic:       t.Topic,
			GroupID:     sub.opts.ConsumerGroup,
			MinBytes:    k.config.MinBytes,
			MaxBytes:    k.config.MaxBytes,
			MaxWait:     k.config.MaxWait,
			StartOffset: kafka.LastOffset,
		})
		readers = append(readers, reader)
	}
	sub.reader = nil
	sub.readers = readers
	if sub.baseCtx == nil {
		sub.baseCtx = context.Background()
	}
	sub.ctx, sub.cancel = context.WithCancel(sub.baseCtx)

	sub.wg.Add(1)
	go func() {
		defer sub.wg.Done()
		idx := 0
		for {
			select {
			case <-sub.ctx.Done():
				return
			default:
			}
			if k.paused.Load() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if sub.limiter != nil {
				if err := sub.limiter.Acquire(sub.ctx); err != nil {
					return
				}
			}
			choice := schedule[idx%len(schedule)]
			idx++
			reader := readers[choice]
			msg, err := reader.FetchMessage(sub.ctx)
			if err != nil {
				if sub.limiter != nil {
					sub.limiter.Release()
				}
				if errors.Is(err, context.Canceled) {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			sub.wg.Add(1)
			go func(m kafka.Message, r *kafka.Reader) {
				defer sub.wg.Done()
				k.handleMessageWithLimiter(sub, r, m)
			}(msg, reader)
		}
	}()
	return nil
}

// Stop stops all consumers gracefully.
func (k *KafkaQueue) Stop() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, sub := range k.subscriptions {
		if sub.cancel != nil {
			sub.cancel()
		}
	}
	for _, sub := range k.subscriptions {
		sub.wg.Wait()
		if sub.reader != nil {
			_ = sub.reader.Close()
		}
		for _, reader := range sub.readers {
			_ = reader.Close()
		}
	}
	k.started = false
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
	conn, err := k.dialer.DialContext(ctx, "tcp", k.config.Brokers[0])
	if err != nil {
		return err
	}
	return conn.Close()
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
	return k.writer.Close()
}

func (k *KafkaQueue) startSubscription(sub *kafkaSubscription) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.config.Brokers,
		Topic:       sub.topic,
		GroupID:     sub.opts.ConsumerGroup,
		MinBytes:    k.config.MinBytes,
		MaxBytes:    k.config.MaxBytes,
		MaxWait:     k.config.MaxWait,
		StartOffset: kafka.LastOffset,
	})
	sub.reader = reader
	if sub.baseCtx == nil {
		sub.baseCtx = context.Background()
	}
	sub.ctx, sub.cancel = context.WithCancel(sub.baseCtx)

	msgCh := make(chan kafka.Message, sub.opts.Concurrency*sub.opts.PrefetchCount)
	sub.wg.Add(1)
	go func() {
		defer sub.wg.Done()
		defer close(msgCh)
		for {
			select {
			case <-sub.ctx.Done():
				return
			default:
			}
			if k.paused.Load() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			msg, err := reader.FetchMessage(sub.ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			msgCh <- msg
		}
	}()

	workerCount := sub.opts.Concurrency
	if workerCount <= 0 {
		workerCount = 1
	}
	for i := 0; i < workerCount; i++ {
		sub.wg.Add(1)
		go func() {
			defer sub.wg.Done()
			for msg := range msgCh {
				k.handleMessage(sub, msg)
			}
		}()
	}
	return nil
}

func (k *KafkaQueue) handleMessage(sub *kafkaSubscription, msg kafka.Message) {
	k.handleMessageWithLimiter(sub, sub.reader, msg)
}

func (k *KafkaQueue) handleMessageWithLimiter(sub *kafkaSubscription, reader *kafka.Reader, msg kafka.Message) {
	defer func() {
		if sub.limiter != nil {
			sub.limiter.Release()
		}
	}()
	m := fromKafkaMessage(msg)
	if m.MaxRetries == 0 {
		m.MaxRetries = sub.opts.MaxRetries
	}
	if m.Expiration == 0 && sub.opts.MessageTTL > 0 {
		m.Expiration = sub.opts.MessageTTL
	}
	if m.Expiration > 0 && !m.Timestamp.IsZero() {
		if time.Since(m.Timestamp) > m.Expiration {
			_ = reader.CommitMessages(sub.ctx, msg)
			return
		}
	}

	for {
		if err := sub.handler(sub.ctx, m); err == nil {
			_ = reader.CommitMessages(sub.ctx, msg)
			return
		}
		m.RetryCount++
		if m.RetryCount > m.MaxRetries {
			if sub.opts.DeadLetterTopic != "" {
				_ = k.Publish(sub.ctx, sub.opts.DeadLetterTopic, m)
			}
			_ = reader.CommitMessages(sub.ctx, msg)
			return
		}
		time.Sleep(sub.opts.RetryDelay)
	}
}

func buildWeightedSchedule(topics []WeightedTopic) []int {
	schedule := make([]int, 0, len(topics))
	for idx, t := range topics {
		if t.Weight <= 0 {
			continue
		}
		for i := 0; i < t.Weight; i++ {
			schedule = append(schedule, idx)
		}
	}
	return schedule
}

func encodeWeightedTopics(topics []WeightedTopic) string {
	var b strings.Builder
	for i, t := range topics {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(t.Topic)
		b.WriteString(":")
		b.WriteString(strconv.Itoa(t.Weight))
	}
	return b.String()
}

func decodeWeightedTopics(raw string) []WeightedTopic {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	topics := make([]WeightedTopic, 0, len(parts))
	for _, part := range parts {
		item := strings.SplitN(part, ":", 2)
		if len(item) != 2 {
			continue
		}
		weight, err := strconv.Atoi(item[1])
		if err != nil || weight <= 0 {
			continue
		}
		topics = append(topics, WeightedTopic{Topic: item[0], Weight: weight})
	}
	return topics
}

func toKafkaMessage(topic string, message *Message) kafka.Message {
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	headers := make([]kafka.Header, 0, len(message.Headers)+6)
	for k, v := range message.Headers {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}
	if message.ID != "" {
		headers = append(headers, kafka.Header{Key: headerID, Value: []byte(message.ID)})
	}
	if !message.Timestamp.IsZero() {
		headers = append(headers, kafka.Header{Key: headerTimestamp, Value: []byte(message.Timestamp.Format(time.RFC3339Nano))})
	}
	if message.Priority != 0 {
		headers = append(headers, kafka.Header{Key: headerPriority, Value: []byte(strconv.Itoa(int(message.Priority)))})
	}
	if message.RetryCount != 0 {
		headers = append(headers, kafka.Header{Key: headerRetryCount, Value: []byte(strconv.Itoa(message.RetryCount))})
	}
	if message.MaxRetries != 0 {
		headers = append(headers, kafka.Header{Key: headerMaxRetries, Value: []byte(strconv.Itoa(message.MaxRetries))})
	}
	if message.Expiration > 0 {
		headers = append(headers, kafka.Header{Key: headerExpiration, Value: []byte(strconv.FormatInt(message.Expiration.Milliseconds(), 10))})
	}

	msg := kafka.Message{
		Topic:   topic,
		Key:     []byte(message.ID),
		Value:   message.Body,
		Headers: headers,
		Time:    message.Timestamp,
	}
	return msg
}

func fromKafkaMessage(msg kafka.Message) *Message {
	m := &Message{
		Body:      msg.Value,
		Headers:   make(map[string]string),
		Timestamp: msg.Time,
	}
	for _, h := range msg.Headers {
		switch h.Key {
		case headerID:
			m.ID = string(h.Value)
		case headerTimestamp:
			if ts, err := time.Parse(time.RFC3339Nano, string(h.Value)); err == nil {
				m.Timestamp = ts
			}
		case headerPriority:
			if v, err := strconv.Atoi(string(h.Value)); err == nil && v >= 0 && v <= 255 {
				m.Priority = uint8(v)
			}
		case headerRetryCount:
			if v, err := strconv.Atoi(string(h.Value)); err == nil && v >= 0 {
				m.RetryCount = v
			}
		case headerMaxRetries:
			if v, err := strconv.Atoi(string(h.Value)); err == nil && v >= 0 {
				m.MaxRetries = v
			}
		case headerExpiration:
			if v, err := strconv.ParseInt(string(h.Value), 10, 64); err == nil && v > 0 {
				m.Expiration = time.Duration(v) * time.Millisecond
			}
		default:
			m.Headers[h.Key] = string(h.Value)
		}
	}
	if m.ID == "" {
		m.ID = string(msg.Key)
	}
	return m
}
