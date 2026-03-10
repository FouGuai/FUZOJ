package repository

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/model"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/sandbox/result"

	"github.com/zeromicro/go-zero/core/logx"
)

// FinalStatusBatcher batches final status persistence and Kafka publishing.
type FinalStatusBatcher struct {
	model        model.SubmissionsModel
	publisher    StatusEventPublisher
	batchSize    int
	interval     time.Duration
	flushTimeout time.Duration

	mu       sync.Mutex
	buffer   []queuedFinalStatus
	signalCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type queuedFinalStatus struct {
	status     pmodel.JudgeStatusResponse
	enqueuedAt time.Time
}

const (
	finalStatusDiagnosticsInterval = 5 * time.Second
	finalStatusQueueWarnThreshold  = 500 * time.Millisecond
	finalStatusFlushWarnThreshold  = 500 * time.Millisecond
)

func NewFinalStatusBatcher(submissionsModel model.SubmissionsModel, publisher StatusEventPublisher, batchSize int, interval, flushTimeout time.Duration) *FinalStatusBatcher {
	if batchSize <= 0 {
		batchSize = 100
	}
	if interval <= 0 {
		interval = time.Second
	}
	if flushTimeout <= 0 {
		flushTimeout = 3 * time.Second
	}
	return &FinalStatusBatcher{
		model:        submissionsModel,
		publisher:    publisher,
		batchSize:    batchSize,
		interval:     interval,
		flushTimeout: flushTimeout,
		signalCh:     make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

func (b *FinalStatusBatcher) Start() {
	go b.run()
}

func (b *FinalStatusBatcher) Stop() {
	close(b.stopCh)
	<-b.doneCh
}

func (b *FinalStatusBatcher) Enqueue(ctx context.Context, status pmodel.JudgeStatusResponse) error {
	logger := logx.WithContext(ctx)
	if status.SubmissionID == "" {
		logger.Error("submission_id is required")
		return appErr.ValidationError("submission_id", "required")
	}
	if !isFinalStatus(status.Status) {
		logger.Error("status must be final")
		return appErr.ValidationError("status", "final_required")
	}
	b.mu.Lock()
	b.buffer = append(b.buffer, queuedFinalStatus{
		status:     status,
		enqueuedAt: time.Now(),
	})
	// Trigger an async flush when the first item arrives to avoid waiting for ticker.
	// Keep the batch-size trigger for high-throughput bursts.
	shouldSignal := len(b.buffer) == 1 || len(b.buffer) >= b.batchSize
	b.mu.Unlock()
	if shouldSignal {
		select {
		case b.signalCh <- struct{}{}:
		default:
		}
	}
	return nil
}

func (b *FinalStatusBatcher) run() {
	ticker := time.NewTicker(b.interval)
	diagTicker := time.NewTicker(finalStatusDiagnosticsInterval)
	defer func() {
		ticker.Stop()
		diagTicker.Stop()
		b.flush(context.Background())
		close(b.doneCh)
	}()
	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.flush(context.Background())
		case <-b.signalCh:
			b.flush(context.Background())
		case <-diagTicker.C:
			b.logHealth(context.Background())
		}
	}
}

func (b *FinalStatusBatcher) flush(ctx context.Context) {
	flushStart := time.Now()
	items := b.drain()
	if len(items) == 0 {
		return
	}

	records := make([]model.FinalStatusRecord, 0, len(items))
	var maxQueueWait time.Duration
	var totalQueueWait time.Duration
	for _, item := range items {
		queueWait := flushStart.Sub(item.enqueuedAt)
		if queueWait > maxQueueWait {
			maxQueueWait = queueWait
		}
		totalQueueWait += queueWait

		clean := stripStatusLogs(item.status)
		payload, err := json.Marshal(clean)
		if err != nil {
			logx.WithContext(ctx).Errorf("marshal final status failed: %v", err)
			continue
		}
		finishedAt := time.Now()
		if item.status.Timestamps.FinishedAt > 0 {
			finishedAt = time.Unix(item.status.Timestamps.FinishedAt, 0)
		}
		records = append(records, model.FinalStatusRecord{
			SubmissionID: item.status.SubmissionID,
			Payload:      string(payload),
			FinishedAt:   finishedAt,
		})
	}
	if len(records) == 0 {
		return
	}
	if b.model == nil {
		logx.WithContext(ctx).Error("submissions model is not configured")
		b.requeue(items)
		return
	}
	dbStart := time.Now()
	dbCtx := ctx
	if b.flushTimeout > 0 {
		var cancel context.CancelFunc
		dbCtx, cancel = context.WithTimeout(ctx, b.flushTimeout)
		defer cancel()
	}
	if err := b.model.UpdateFinalStatusBatch(dbCtx, records); err != nil {
		logx.WithContext(ctx).Errorf("batch store final status failed: %v", err)
		b.requeue(items)
		return
	}
	dbDuration := time.Since(dbStart)
	failedPublishes := make([]pmodel.JudgeStatusResponse, 0)
	publishStart := time.Now()
	for _, item := range items {
		if b.publisher == nil {
			continue
		}
		pubCtx := ctx
		if b.flushTimeout > 0 {
			var cancel context.CancelFunc
			pubCtx, cancel = context.WithTimeout(context.Background(), b.flushTimeout)
			err := b.publisher.PublishFinalStatus(pubCtx, item.status)
			cancel()
			if err != nil {
				logx.WithContext(ctx).Errorf("publish final status failed: %v", err)
				failedPublishes = append(failedPublishes, item.status)
			}
			continue
		}
		if err := b.publisher.PublishFinalStatus(pubCtx, item.status); err != nil {
			logx.WithContext(ctx).Errorf("publish final status failed: %v", err)
			failedPublishes = append(failedPublishes, item.status)
		}
	}
	publishDuration := time.Since(publishStart)
	if len(failedPublishes) > 0 {
		b.requeue(wrapQueuedFinalStatuses(failedPublishes))
		select {
		case b.signalCh <- struct{}{}:
		default:
		}
	}
	totalDuration := time.Since(flushStart)
	avgQueueWait := time.Duration(0)
	if len(items) > 0 {
		avgQueueWait = totalQueueWait / time.Duration(len(items))
	}
	if maxQueueWait >= finalStatusQueueWarnThreshold || totalDuration >= finalStatusFlushWarnThreshold || len(failedPublishes) > 0 {
		logx.WithContext(ctx).Infof("final status flush stats size=%d failed=%d queue_wait_max=%s queue_wait_avg=%s db_cost=%s publish_cost=%s total_cost=%s",
			len(items), len(failedPublishes), maxQueueWait, avgQueueWait, dbDuration, publishDuration, totalDuration)
	}
}

func (b *FinalStatusBatcher) drain() []queuedFinalStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffer) == 0 {
		return nil
	}
	items := make([]queuedFinalStatus, len(b.buffer))
	copy(items, b.buffer)
	b.buffer = b.buffer[:0]
	return items
}

func (b *FinalStatusBatcher) requeue(items []queuedFinalStatus) {
	if len(items) == 0 {
		return
	}
	b.mu.Lock()
	b.buffer = append(items, b.buffer...)
	b.mu.Unlock()
}

func (b *FinalStatusBatcher) logHealth(ctx context.Context) {
	buffered := b.bufferedLen()
	if buffered == 0 {
		return
	}
	if buffered >= b.batchSize*3 {
		logx.WithContext(ctx).Errorf("final status backlog detected buffered=%d batch_size=%d interval=%s", buffered, b.batchSize, b.interval)
		return
	}
	logx.WithContext(ctx).Infof("final status backlog snapshot buffered=%d batch_size=%d interval=%s", buffered, b.batchSize, b.interval)
}

func (b *FinalStatusBatcher) bufferedLen() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buffer)
}

func wrapQueuedFinalStatuses(items []pmodel.JudgeStatusResponse) []queuedFinalStatus {
	if len(items) == 0 {
		return nil
	}
	out := make([]queuedFinalStatus, 0, len(items))
	now := time.Now()
	for _, item := range items {
		out = append(out, queuedFinalStatus{
			status:     item,
			enqueuedAt: now,
		})
	}
	return out
}

func stripStatusLogs(status pmodel.JudgeStatusResponse) pmodel.JudgeStatusResponse {
	clean := status
	if clean.Compile != nil {
		compile := *clean.Compile
		compile.Log = ""
		compile.Error = ""
		clean.Compile = &compile
	}
	if len(clean.Tests) == 0 {
		return clean
	}
	tests := make([]result.TestcaseResult, 0, len(clean.Tests))
	for _, test := range clean.Tests {
		item := test
		item.RuntimeLog = ""
		item.CheckerLog = ""
		item.Stdout = ""
		item.Stderr = ""
		tests = append(tests, item)
	}
	clean.Tests = tests
	return clean
}
