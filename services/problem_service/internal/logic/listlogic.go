// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"strconv"

	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/logic/problem_app"
	"fuzoj/services/problem_service/internal/svc"
	"fuzoj/services/problem_service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

const (
	defaultProblemListLimit = 20
	maxProblemListLimit     = 100
)

type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLogic) List(req *types.ListProblemsRequest) (resp *types.ListProblemsResponse, err error) {
	if req == nil {
		return nil, pkgerrors.ValidationError("request", "required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = defaultProblemListLimit
	}
	if limit > maxProblemListLimit {
		limit = maxProblemListLimit
	}
	var cursorID int64
	if req.Cursor != "" {
		cursorID, err = strconv.ParseInt(req.Cursor, 10, 64)
		if err != nil || cursorID <= 0 {
			return nil, pkgerrors.ValidationError("cursor", "must be a positive integer")
		}
	}
	manager := problem_app.NewProblemAppFromContext(l.svcCtx)
	items, hasMore, err := manager.ListPublishedProblems(l.ctx, cursorID, limit)
	if err != nil {
		return nil, err
	}
	return buildListProblemsResponse(l.ctx, items, hasMore), nil
}
