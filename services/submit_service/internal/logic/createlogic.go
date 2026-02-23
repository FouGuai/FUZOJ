// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"

	"fuzoj/services/submit_service/internal/logic/submit_app"
	"fuzoj/services/submit_service/internal/svc"
	"fuzoj/services/submit_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateLogic) Create(req *types.CreateSubmissionRequest) (resp *types.CreateSubmissionResponse, err error) {
	app, err := submit_app.NewSubmitApp(l.svcCtx)
	if err != nil {
		return nil, err
	}
	submissionID, status, err := app.Submit(l.ctx, submit_app.SubmitParams{
		ProblemID:         req.ProblemId,
		UserID:            req.UserId,
		LanguageID:        req.LanguageId,
		SourceCode:        req.SourceCode,
		ContestID:         req.ContestId,
		Scene:             req.Scene,
		ExtraCompileFlags: req.ExtraCompileFlags,
		IdempotencyKey:    req.IdempotencyKey,
		ClientIP:          getClientIP(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	return buildCreateResponse(l.ctx, submissionID, status), nil
}
