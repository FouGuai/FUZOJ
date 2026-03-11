package weighted_kq

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

const (
	dispatchMetricsInterval  = 5 * time.Second
	defaultTopicQueueFactor  = 4
	defaultTopicQueueMinSize = 64
	defaultRetryCapDivisor   = 8
)

// WeightedKqOptions defines weighted Kafka queue settings.
type WeightedKqOptions struct {
	Brokers          []string
	Group            string
	Topics           []string
	TopicWeights     map[string]int
	ConsumersTotal   int
	ProcessorsTotal  int
	MinBytes         int
	MaxBytes         int
	ServiceName      string
	RetryTopic       string
	RetryMaxInFlight int
	AutoAddRetry     bool
}

// WeightedQueuePolicy defines runtime dispatch behavior.
type WeightedQueuePolicy struct {
	TopicWeights     map[string]int
	RetryTopic       string
	RetryMaxInFlight int
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
	processorsPerTopic := maxInt(1, opts.ProcessorsTotal)

	confs := make([]kq.KqConf, 0, len(topics))
	for _, topic := range topics {
		conf := kq.KqConf{
			Brokers:   opts.Brokers,
			Group:     defaultGroup(opts.Group),
			Topic:     topic,
			Consumers: consumers[topic],
			// Use a shared downstream worker pool. Per-topic processor count here
			// is only for ingesting messages into the dispatcher and waiting result.
			Processors: processorsPerTopic,
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
	return NewWeightedKqQueuesWithPolicy(confs, handler, WeightedQueuePolicy{}, opts...)
}

// NewWeightedKqQueuesWithPolicy creates a group queue with shared weighted dispatch.
func NewWeightedKqQueuesWithPolicy(confs []kq.KqConf, handler kq.ConsumeHandler, policy WeightedQueuePolicy, opts ...kq.QueueOption) (queue.MessageQueue, error) {
	if len(confs) == 0 {
		return nil, errors.New("kq configs are required")
	}
	if handler == nil {
		return nil, errors.New("consume handler is required")
	}

	topics := make([]string, 0, len(confs))
	weights := make(map[string]int, len(confs))
	workers := 1
	for _, conf := range confs {
		topics = append(topics, conf.Topic)
		if policy.TopicWeights != nil {
			weights[conf.Topic] = maxInt(1, policy.TopicWeights[conf.Topic])
		} else {
			weights[conf.Topic] = maxInt(1, conf.Consumers)
		}
		if conf.Processors > workers {
			workers = conf.Processors
		}
	}
	if workers <= 0 {
		workers = 1
	}
	retryCap := policy.RetryMaxInFlight
	if policy.RetryTopic != "" && retryCap <= 0 {
		retryCap = maxInt(1, workers/defaultRetryCapDivisor)
	}

	dispatcher := newSharedDispatcher(sharedDispatcherOptions{
		topics:     topics,
		weights:    weights,
		workers:    workers,
		retryTopic: policy.RetryTopic,
		retryCap:   retryCap,
		handler:    handler,
	})

	queues := make([]queue.MessageQueue, 0, len(confs))
	for _, conf := range confs {
		topicHandler := &topicDispatchHandler{topic: conf.Topic, dispatcher: dispatcher}
		q, err := kq.NewQueue(conf, topicHandler, opts...)
		if err != nil {
			return nil, err
		}
		queues = append(queues, q)
	}
	return &queueGroup{queues: queues, dispatcher: dispatcher}, nil
}

type queueGroup struct {
	queues     []queue.MessageQueue
	dispatcher *sharedDispatcher
}

func (g *queueGroup) Start() {
	if g.dispatcher != nil {
		g.dispatcher.Start()
		defer g.dispatcher.Stop()
	}
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
	if g.dispatcher != nil {
		g.dispatcher.Stop()
	}
}

type topicDispatchHandler struct {
	topic      string
	dispatcher *sharedDispatcher
}

func (h *topicDispatchHandler) Consume(ctx context.Context, key, value string) error {
	if h == nil || h.dispatcher == nil {
		return errors.New("dispatcher is not configured")
	}
	return h.dispatcher.Submit(ctx, h.topic, key, value)
}

type dispatchTask struct {
	ctx        context.Context
	topic      string
	key        string
	value      string
	enqueuedAt time.Time
	done       chan error
}

type topicWindowStats struct {
	dispatched int64
	successes  int64
	failures   int64
	waitTotal  time.Duration
	waitMax    time.Duration
}

type dispatcherStats struct {
	mu      sync.Mutex
	byTopic map[string]*topicWindowStats
}

func newDispatcherStats(topics []string) *dispatcherStats {
	stats := &dispatcherStats{byTopic: make(map[string]*topicWindowStats, len(topics))}
	for _, topic := range topics {
		stats.byTopic[topic] = &topicWindowStats{}
	}
	return stats
}

func (s *dispatcherStats) recordDispatch(topic string, wait time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureTopic(topic)
	item.dispatched++
	item.waitTotal += wait
	if wait > item.waitMax {
		item.waitMax = wait
	}
}

func (s *dispatcherStats) recordResult(topic string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := s.ensureTopic(topic)
	if err != nil {
		item.failures++
		return
	}
	item.successes++
}

func (s *dispatcherStats) snapshotAndReset() map[string]topicWindowStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]topicWindowStats, len(s.byTopic))
	for topic, item := range s.byTopic {
		out[topic] = *item
		*item = topicWindowStats{}
	}
	return out
}

func (s *dispatcherStats) ensureTopic(topic string) *topicWindowStats {
	item, ok := s.byTopic[topic]
	if ok {
		return item
	}
	item = &topicWindowStats{}
	s.byTopic[topic] = item
	return item
}

type sharedDispatcherOptions struct {
	topics     []string
	weights    map[string]int
	workers    int
	retryTopic string
	retryCap   int
	handler    kq.ConsumeHandler
}

type sharedDispatcher struct {
	handler       kq.ConsumeHandler
	workers       int
	retryTopic    string
	retryCap      int
	topicQueues   map[string]chan *dispatchTask
	schedule      []string
	wakeCh        chan struct{}
	jobs          chan *dispatchTask
	stopCh        chan struct{}
	startOnce     sync.Once
	stopOnce      sync.Once
	wg            sync.WaitGroup
	cursor        int
	retryInFlight int64
	stats         *dispatcherStats
}

func newSharedDispatcher(opts sharedDispatcherOptions) *sharedDispatcher {
	workers := maxInt(1, opts.workers)
	queueSize := maxInt(defaultTopicQueueMinSize, workers*defaultTopicQueueFactor)
	topicQueues := make(map[string]chan *dispatchTask, len(opts.topics))
	for _, topic := range opts.topics {
		topicQueues[topic] = make(chan *dispatchTask, queueSize)
	}
	weights := make(map[string]int, len(opts.topics))
	for _, topic := range opts.topics {
		weights[topic] = maxInt(1, opts.weights[topic])
	}
	schedule := buildDispatchSchedule(opts.topics, weights)
	return &sharedDispatcher{
		handler:     opts.handler,
		workers:     workers,
		retryTopic:  opts.retryTopic,
		retryCap:    maxInt(0, opts.retryCap),
		topicQueues: topicQueues,
		schedule:    schedule,
		wakeCh:      make(chan struct{}, 1),
		jobs:        make(chan *dispatchTask, workers*2),
		stopCh:      make(chan struct{}),
		stats:       newDispatcherStats(opts.topics),
	}
}

func (d *sharedDispatcher) Start() {
	d.startOnce.Do(func() {
		for i := 0; i < d.workers; i++ {
			d.wg.Add(1)
			go d.runWorker()
		}
		d.wg.Add(1)
		go d.runDispatch()
		d.wg.Add(1)
		go d.runMetrics()
		logx.Infof("weighted kq dispatcher started workers=%d retry_topic=%s retry_cap=%d", d.workers, d.retryTopic, d.retryCap)
	})
}

func (d *sharedDispatcher) Stop() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		d.wg.Wait()
		logx.Info("weighted kq dispatcher stopped")
	})
}

func (d *sharedDispatcher) Submit(ctx context.Context, topic, key, value string) error {
	topicCh, ok := d.topicQueues[topic]
	if !ok {
		return fmt.Errorf("unknown topic: %s", topic)
	}
	task := &dispatchTask{
		ctx:        ctx,
		topic:      topic,
		key:        key,
		value:      value,
		enqueuedAt: time.Now(),
		done:       make(chan error, 1),
	}
	select {
	case <-d.stopCh:
		return errors.New("weighted dispatcher is stopped")
	case topicCh <- task:
		d.signalWake()
	}

	select {
	case <-d.stopCh:
		return errors.New("weighted dispatcher is stopped")
	case err := <-task.done:
		return err
	}
}

func (d *sharedDispatcher) runDispatch() {
	defer d.wg.Done()
	for {
		if d.dispatchPending() {
			continue
		}
		select {
		case <-d.stopCh:
			return
		case <-d.wakeCh:
		}
	}
}

func (d *sharedDispatcher) dispatchPending() bool {
	found := false
	for {
		task, ok := d.nextTask()
		if !ok {
			return found
		}
		found = true
		select {
		case <-d.stopCh:
			return false
		case d.jobs <- task:
			wait := time.Since(task.enqueuedAt)
			d.stats.recordDispatch(task.topic, wait)
		}
	}
}

func (d *sharedDispatcher) nextTask() (*dispatchTask, bool) {
	if len(d.schedule) == 0 {
		return nil, false
	}
	for i := 0; i < len(d.schedule); i++ {
		topic := d.schedule[d.cursor]
		d.cursor = (d.cursor + 1) % len(d.schedule)
		if d.retryTopic != "" && topic == d.retryTopic && d.retryCap > 0 && int(atomic.LoadInt64(&d.retryInFlight)) >= d.retryCap {
			continue
		}
		ch := d.topicQueues[topic]
		select {
		case task := <-ch:
			if d.retryTopic != "" && topic == d.retryTopic && d.retryCap > 0 {
				atomic.AddInt64(&d.retryInFlight, 1)
			}
			return task, true
		default:
		}
	}
	return nil, false
}

func (d *sharedDispatcher) runWorker() {
	defer d.wg.Done()
	for {
		select {
		case <-d.stopCh:
			return
		case task := <-d.jobs:
			err := d.handler.Consume(task.ctx, task.key, task.value)
			if d.retryTopic != "" && task.topic == d.retryTopic && d.retryCap > 0 {
				atomic.AddInt64(&d.retryInFlight, -1)
			}
			d.stats.recordResult(task.topic, err)
			task.done <- err
		}
	}
}

func (d *sharedDispatcher) runMetrics() {
	defer d.wg.Done()
	ticker := time.NewTicker(dispatchMetricsInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.logMetrics(dispatchMetricsInterval)
		}
	}
}

func (d *sharedDispatcher) logMetrics(window time.Duration) {
	statsByTopic := d.stats.snapshotAndReset()
	windowSeconds := window.Seconds()
	if windowSeconds <= 0 {
		windowSeconds = 1
	}
	for topic, stats := range statsByTopic {
		attempts := stats.successes + stats.failures
		backlog := len(d.topicQueues[topic])
		avgWait := time.Duration(0)
		if stats.dispatched > 0 {
			avgWait = time.Duration(int64(stats.waitTotal) / stats.dispatched)
		}
		logx.Infof(
			"weighted kq topic metrics window=%s topic=%s dispatched=%d attempts=%d successes=%d failures=%d backlog=%d attempt_qps=%.2f success_qps=%.2f wait_avg=%s wait_max=%s retry_inflight=%d retry_cap=%d",
			window,
			topic,
			stats.dispatched,
			attempts,
			stats.successes,
			stats.failures,
			backlog,
			float64(attempts)/windowSeconds,
			float64(stats.successes)/windowSeconds,
			avgWait,
			stats.waitMax,
			atomic.LoadInt64(&d.retryInFlight),
			d.retryCap,
		)
	}
}

func (d *sharedDispatcher) signalWake() {
	select {
	case d.wakeCh <- struct{}{}:
	default:
	}
}

func buildDispatchSchedule(topics []string, weights map[string]int) []string {
	schedule := make([]string, 0, len(topics))
	for _, topic := range topics {
		weight := maxInt(1, weights[topic])
		for i := 0; i < weight; i++ {
			schedule = append(schedule, topic)
		}
	}
	return schedule
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
