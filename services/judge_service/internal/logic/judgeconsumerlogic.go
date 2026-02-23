// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"encoding/json"
	"time"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/logic/judge_app"
	"fuzoj/services/judge_service/internal/pmodel"
	"fuzoj/services/judge_service/internal/svc"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

type JudgeConsumerLogic struct {
	logx.Logger
	ctx        context.Context
	svcCtx     *svc.ServiceContext
	processor  *judge_app.JudgeApp
	maxRetries int
	retryDelay time.Duration
	messageTTL time.Duration
	deadLetter *kq.Pusher
}

func NewJudgeConsumerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JudgeConsumerLogic {
	processor := newJudgeAppFromSvc(svcCtx)
	if svcCtx == nil {
		return &JudgeConsumerLogic{
			Logger:    logx.WithContext(ctx),
			ctx:       ctx,
			svcCtx:    svcCtx,
			processor: processor,
		}
	}
	return &JudgeConsumerLogic{
		Logger:     logx.WithContext(ctx),
		ctx:        ctx,
		svcCtx:     svcCtx,
		processor:  processor,
		maxRetries: svcCtx.Config.Kafka.MaxRetries,
		retryDelay: svcCtx.Config.Kafka.RetryDelay,
		messageTTL: svcCtx.Config.Kafka.MessageTTL,
		deadLetter: svcCtx.DeadLetterPusher,
	}
}

func (l *JudgeConsumerLogic) Consume(ctx context.Context, key, value string) error {
	l.Infof("handle judge message start")
	if l.processor == nil {
		l.Error("judge processor is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge service is not configured")
	}
	if ctx == nil {
		ctx = l.ctx
	}
	if value == "" {
		return nil
	}
	var payload pmodel.JudgeMessage
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		l.Errorf("decode judge message failed: %v", err)
		return nil
	}
	if payload.SubmissionID == "" {
		l.Error("submission_id is required")
		return nil
	}
	if l.messageTTL > 0 && payload.CreatedAt > 0 {
		createdAt := time.Unix(payload.CreatedAt, 0)
		if time.Since(createdAt) > l.messageTTL {
			l.Infof("skip expired judge message submission_id=%s", payload.SubmissionID)
			return nil
		}
	}
	maxRetries := l.maxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	retryDelay := l.retryDelay
	if retryDelay <= 0 {
		retryDelay = time.Second
	}
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := l.processor.HandleMessage(ctx, payload); err == nil {
			return nil
		} else if attempt >= maxRetries {
			if l.deadLetter != nil {
				if err := l.deadLetter.PushWithKey(ctx, payload.SubmissionID, value); err != nil {
					l.Errorf("dead letter publish failed: %v", err)
				}
			}
			l.Errorf("judge message failed after retries submission_id=%s", payload.SubmissionID)
			return nil
		}
		time.Sleep(retryDelay)
	}
	return nil
}

func newJudgeAppFromSvc(svcCtx *svc.ServiceContext) *judge_app.JudgeApp {
	if svcCtx == nil {
		return nil
	}
	cfg := judge_app.JudgeAppConfig{
		Worker:         svcCtx.Worker,
		StatusRepo:     svcCtx.StatusRepo,
		ProblemClient:  svcCtx.ProblemClient,
		DataCache:      svcCtx.DataCache,
		Storage:        svcCtx.Storage,
		RetryPusher:    svcCtx.RetryPusher,
		DeadPusher:     svcCtx.DeadLetterPusher,
		SourceBucket:   svcCtx.Config.Source.Bucket,
		WorkRoot:       svcCtx.Config.Judge.WorkRoot,
		WorkerTimeout:  svcCtx.Config.Worker.Timeout,
		ProblemTimeout: svcCtx.Config.Problem.Timeout,
		StorageTimeout: svcCtx.Config.Source.Timeout,
		StatusTimeout:  svcCtx.Config.Status.Timeout,
		MetaTTL:        svcCtx.Config.Problem.MetaTTL,
		WorkerPoolSize: svcCtx.Config.Worker.PoolSize,
		RetryTopic:     svcCtx.Config.Kafka.RetryTopic,
		PoolRetryMax:   svcCtx.Config.Kafka.PoolRetryMax,
		PoolRetryBase:  svcCtx.Config.Kafka.PoolRetryBase,
		PoolRetryMaxD:  svcCtx.Config.Kafka.PoolRetryMaxD,
		DeadLetter:     svcCtx.Config.Kafka.DeadLetter,
	}
	processor, err := judge_app.NewJudgeApp(cfg)
	if err != nil {
		logx.Errorf("init judge processor failed: %v", err)
		return nil
	}
	return processor
}
