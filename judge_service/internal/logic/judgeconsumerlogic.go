// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/internal/common/mq"
	"fuzoj/judge_service/internal/svc"
	appErr "fuzoj/pkg/errors"

	"github.com/zeromicro/go-zero/core/logx"
)

type JudgeConsumerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewJudgeConsumerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *JudgeConsumerLogic {
	return &JudgeConsumerLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *JudgeConsumerLogic) HandleMessage(msg *mq.Message) error {
	l.Infof("handle judge message start")
	if l.svcCtx == nil || l.svcCtx.JudgeService == nil {
		l.Error("judge service is not configured")
		return appErr.New(appErr.ServiceUnavailable).WithMessage("judge service is not configured")
	}
	return l.svcCtx.JudgeService.HandleMessage(l.ctx, msg)
}
