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
	model         model.SubmissionsModel
	publisher     StatusEventPublisher
	batchSize     int
	interval      time.Duration
	flushTimeout  time.Duration

	mu       sync.Mutex
	buffer   []pmodel.JudgeStatusResponse
	signalCh chan struct{}
	stopCh   chan struct{}
	doneCh   chan struct{}
}

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
	b.buffer = append(b.buffer, status)
	shouldSignal := len(b.buffer) >= b.batchSize
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
	defer func() {
		ticker.Stop()
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
		}
	}
}

func (b *FinalStatusBatcher) flush(ctx context.Context) {
	items := b.drain()
	if len(items) == 0 {
		return
	}
	ctxFlush := ctx
	if b.flushTimeout > 0 {
		var cancel context.CancelFunc
		ctxFlush, cancel = context.WithTimeout(ctx, b.flushTimeout)
		defer cancel()
	}

	records := make([]model.FinalStatusRecord, 0, len(items))
	for _, item := range items {
		clean := stripStatusLogs(item)
		payload, err := json.Marshal(clean)
		if err != nil {
			logx.WithContext(ctxFlush).Errorf("marshal final status failed: %v", err)
			continue
		}
		finishedAt := time.Now()
		if item.Timestamps.FinishedAt > 0 {
			finishedAt = time.Unix(item.Timestamps.FinishedAt, 0)
		}
		records = append(records, model.FinalStatusRecord{
			SubmissionID: item.SubmissionID,
			Payload:      string(payload),
			FinishedAt:   finishedAt,
		})
	}
	if len(records) == 0 {
		return
	}
	if b.model == nil {
		logx.WithContext(ctxFlush).Error("submissions model is not configured")
		b.requeue(items)
		return
	}
	if err := b.model.UpdateFinalStatusBatch(ctxFlush, records); err != nil {
		logx.WithContext(ctxFlush).Errorf("batch store final status failed: %v", err)
		b.requeue(items)
		return
	}
	for _, item := range items {
		if b.publisher == nil {
			continue
		}
		if err := b.publisher.PublishFinalStatus(ctxFlush, item); err != nil {
			logx.WithContext(ctxFlush).Errorf("publish final status failed: %v", err)
		}
	}
}

func (b *FinalStatusBatcher) drain() []pmodel.JudgeStatusResponse {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffer) == 0 {
		return nil
	}
	items := make([]pmodel.JudgeStatusResponse, len(b.buffer))
	copy(items, b.buffer)
	b.buffer = b.buffer[:0]
	return items
}

func (b *FinalStatusBatcher) requeue(items []pmodel.JudgeStatusResponse) {
	if len(items) == 0 {
		return
	}
	b.mu.Lock()
	b.buffer = append(items, b.buffer...)
	b.mu.Unlock()
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
