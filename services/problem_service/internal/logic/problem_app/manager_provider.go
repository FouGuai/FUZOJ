package problem_app

import (
	"context"

	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/svc"
)

// ProblemApp exposes core problem behaviors for internal integrations.
type ProblemApp interface {
	GetLatestMeta(ctx context.Context, problemID int64) (repository.ProblemLatestMeta, error)
}

func NewProblemAppFromContext(svcCtx *svc.ServiceContext) *problemApp {
	if svcCtx == nil {
		return newProblemApp(nil, nil, nil, nil, nil, nil, nil, "", "", 0, 0, 0, 0)
	}
	return newProblemApp(
		svcCtx.ProblemRepo,
		svcCtx.StatementRepo,
		svcCtx.UploadRepo,
		svcCtx.Storage,
		svcCtx.CleanupPublisher,
		svcCtx.MetaPublisher,
		svcCtx.Conn,
		svcCtx.Config.MinIO.Bucket,
		svcCtx.Config.Upload.KeyPrefix,
		svcCtx.Config.Upload.PartSizeBytes,
		svcCtx.Config.Upload.SessionTTL,
		svcCtx.Config.Upload.PresignTTL,
		svcCtx.Config.Statement.MaxBytes,
	)
}

// NewProblemApp exposes the core manager as an interface for other internal packages.
func NewProblemApp(svcCtx *svc.ServiceContext) ProblemApp {
	return NewProblemAppFromContext(svcCtx)
}
