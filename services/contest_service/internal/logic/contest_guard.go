package logic

import (
	"context"
	"errors"

	appErr "fuzoj/pkg/errors"
	"fuzoj/services/contest_service/internal/repository"
	"fuzoj/services/contest_service/internal/svc"
)

func loadContestOrError(ctx context.Context, svcCtx *svc.ServiceContext, contestID string) (repository.ContestDetail, error) {
	detail, err := svcCtx.ContestStore.Get(ctx, contestID)
	if err != nil {
		if errors.Is(err, repository.ErrContestNotFound) {
			return repository.ContestDetail{}, appErr.New(appErr.ContestNotFound)
		}
		return repository.ContestDetail{}, appErr.Wrap(err, appErr.DatabaseError)
	}
	return detail, nil
}
