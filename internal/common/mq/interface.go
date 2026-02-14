package mq

import (
	"context"
	"time"
)

// MessageQueue defines the unified interface for message queue operations.
// This abstraction allows switching between different MQ implementations
// (RabbitMQ, Kafka, NATS) without changing business logic.
type MessageQueue interface {
	Producer
	Consumer

	// Ping verifies the message queue connection is alive
	Ping(ctx context.Context) error

	// Close closes the message queue connection
	Close() error
}

// Producer defines the interface for publishing messages
type Producer interface {
	// Publish publishes a message to the specified topic/queue
	Publish(ctx context.Context, topic string, message *Message) error

	// PublishBatch publishes multiple messages in a batch
	PublishBatch(ctx context.Context, topic string, messages []*Message) error
}

// Consumer defines the interface for consuming messages
type Consumer interface {
	// Subscribe subscribes to a topic/queue and processes messages with the given handler
	// The handler should return nil on success or an error on failure
	Subscribe(ctx context.Context, topic string, handler HandlerFunc) error

	// SubscribeWithOptions subscribes with custom options
	SubscribeWithOptions(ctx context.Context, topic string, handler HandlerFunc, opts *SubscribeOptions) error

	// Start starts consuming messages
	Start() error

	// Stop gracefully stops consuming messages
	Stop() error

	// Pause temporarily pauses message consumption
	Pause() error

	// Resume resumes message consumption after pause
	Resume() error
}

// Message represents a message in the queue
type Message struct {
	// ID is the unique identifier for the message
	ID string `json:"id"`

	// Body is the message payload
	Body []byte `json:"body"`

	// Headers contains metadata about the message
	Headers map[string]string `json:"headers"`

	// Timestamp is when the message was created
	Timestamp time.Time `json:"timestamp"`

	// Priority is the message priority (0-255, 0 is highest)
	Priority uint8 `json:"priority"`

	// Retry information
	RetryCount int `json:"retry_count"`
	MaxRetries int `json:"max_retries"`

	// Expiration time for the message
	Expiration time.Duration `json:"expiration"`
}

// HandlerFunc is the function signature for message handlers
// It receives the message and returns an error if processing failed
type HandlerFunc func(ctx context.Context, message *Message) error

// SubscribeOptions defines options for subscribing to a topic
type SubscribeOptions struct {
	// QueueName is the name of the queue (for RabbitMQ)
	QueueName string

	// ConsumerGroup is the consumer group name (for Kafka)
	ConsumerGroup string

	// PrefetchCount sets the number of messages to prefetch
	// Default: 1 (fair dispatch for judge tasks)
	PrefetchCount int

	// Concurrency sets the number of concurrent workers
	// Default: 1
	Concurrency int

	// MaxRetries sets the maximum number of retries for failed messages
	// Default: 3
	MaxRetries int

	// RetryDelay sets the delay between retries
	// Default: 1 second
	RetryDelay time.Duration

	// DeadLetterTopic is where messages go after max retries
	DeadLetterTopic string

	// MessageTTL sets the time-to-live for messages in the queue
	MessageTTL time.Duration
}

// SetDefaults sets default values for subscribe options
func (o *SubscribeOptions) SetDefaults() {
	if o.PrefetchCount == 0 {
		o.PrefetchCount = 1
	}
	if o.Concurrency == 0 {
		o.Concurrency = 1
	}
	if o.MaxRetries == 0 {
		o.MaxRetries = 3
	}
	if o.RetryDelay == 0 {
		o.RetryDelay = time.Second
	}
}

// NewMessage creates a new message with the given body
func NewMessage(body []byte) *Message {
	return &Message{
		Body:       body,
		Headers:    make(map[string]string),
		Timestamp:  time.Now(),
		RetryCount: 0,
		MaxRetries: 3,
	}
}

// SetHeader sets a header value
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// GetHeader retrieves a header value
func (m *Message) GetHeader(key string) (string, bool) {
	if m.Headers == nil {
		return "", false
	}
	val, ok := m.Headers[key]
	return val, ok
}

// ShouldRetry determines if the message should be retried
func (m *Message) ShouldRetry() bool {
	return m.RetryCount < m.MaxRetries
}

// IncrementRetry increments the retry count
func (m *Message) IncrementRetry() {
	m.RetryCount++
}
