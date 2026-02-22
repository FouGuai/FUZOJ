// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/internal/common/mq"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type JudgeConsumerLogic struct {
	logx.Logger
	ctx       context.Context
	svcCtx    *svc.ServiceContext
	processor *JudgeProcessor
}

func NewJudgeConsumerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JudgeConsumerLogic {
	processor := newJudgeProcessorFromSvc(svcCtx)
	return &JudgeConsumerLogic{
		Logger:    logx.WithContext(ctx),
		ctx:       ctx,
		svcCtx:    svcCtx,
		processor: processor,
	}
}

func (l *JudgeConsumerLogic) HandleMessage(msg *mq.Message) error {
	l.Infof("handle judge message start")
	if l.processor == nil {
		l.Error("judge processor is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge service is not configured")
	}
	return l.processor.HandleMessage(l.ctx, msg)
}

func newJudgeProcessorFromSvc(svcCtx *svc.ServiceContext) *JudgeProcessor {
	if svcCtx == nil {
		return nil
	}
	cfg := JudgeProcessorConfig{
		Worker:         svcCtx.Worker,
		StatusRepo:     svcCtx.StatusRepo,
		ProblemClient:  svcCtx.ProblemClient,
		DataCache:      svcCtx.DataCache,
		Storage:        svcCtx.Storage,
		Queue:          svcCtx.Queue,
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
	processor, err := NewJudgeProcessor(cfg)
	if err != nil {
		logx.Errorf("init judge processor failed: %v", err)
		return nil
	}
	return processor
}
