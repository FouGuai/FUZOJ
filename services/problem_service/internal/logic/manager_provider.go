package logic

import (
	"context"

	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/svc"
)

// ProblemManager exposes core problem behaviors for internal integrations.
type ProblemManager interface {
	GetLatestMeta(ctx context.Context, problemID int64) (repository.ProblemLatestMeta, error)
}

func newProblemManagerFromContext(svcCtx *svc.ServiceContext) *problemManager {
	if svcCtx == nil {
		return newProblemManager(nil, nil, nil, nil, nil, "", "", 0, 0, 0)
	}
	return newProblemManager(
		svcCtx.ProblemRepo,
		svcCtx.UploadRepo,
		svcCtx.Storage,
		svcCtx.CleanupPublisher,
		svcCtx.Conn,
		svcCtx.Config.MinIO.Bucket,
		svcCtx.Config.Upload.KeyPrefix,
		svcCtx.Config.Upload.PartSizeBytes,
		svcCtx.Config.Upload.SessionTTL,
		svcCtx.Config.Upload.PresignTTL,
	)
}

// NewProblemManager exposes the core manager as an interface for other internal packages.
func NewProblemManager(svcCtx *svc.ServiceContext) ProblemManager {
	return newProblemManagerFromContext(svcCtx)
}
