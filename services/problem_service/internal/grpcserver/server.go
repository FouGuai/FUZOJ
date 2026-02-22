package grpcserver

import (
	"context"

	problemv1 "fuzoj/api/gen/problem/v1"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/logic"
	"fuzoj/services/problem_service/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProblemRPCServer implements gRPC problem service.
type ProblemRPCServer struct {
	problemv1.UnimplementedProblemServiceServer
	svcCtx *svc.ServiceContext
}

// NewProblemRPCServer creates a new gRPC server.
func NewProblemRPCServer(svcCtx *svc.ServiceContext) *ProblemRPCServer {
	return &ProblemRPCServer{svcCtx: svcCtx}
}

// GetLatest returns latest published meta.
func (s *ProblemRPCServer) GetLatest(ctx context.Context, req *problemv1.GetLatestRequest) (*problemv1.GetLatestResponse, error) {
	if req == nil || req.GetProblemId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id is required")
	}

	manager := logic.NewProblemManager(s.svcCtx)
	meta, err := manager.GetLatestMeta(ctx, req.GetProblemId())
	if err != nil {
		return nil, mapError(err)
	}

	return &problemv1.GetLatestResponse{
		Meta: &problemv1.ProblemLatestMeta{
			ProblemId:    meta.ProblemID,
			Version:      meta.Version,
			ManifestHash: meta.ManifestHash,
			DataPackKey:  meta.DataPackKey,
			DataPackHash: meta.DataPackHash,
			UpdatedAt:    meta.UpdatedAt.Unix(),
		},
	}, nil
}

func mapError(err error) error {
	code := pkgerrors.GetCode(err)
	switch code {
	case pkgerrors.ProblemNotFound, pkgerrors.NotFound:
		return status.Error(codes.NotFound, code.Message())
	case pkgerrors.InvalidParams:
		return status.Error(codes.InvalidArgument, code.Message())
	case pkgerrors.Unauthorized:
		return status.Error(codes.Unauthenticated, code.Message())
	case pkgerrors.Forbidden:
		return status.Error(codes.PermissionDenied, code.Message())
	default:
		return status.Error(codes.Internal, code.Message())
	}
}
