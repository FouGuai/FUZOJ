package rpc

import (
	"context"

	problemv1 "fuzoj/api/gen/problem/v1"
	"fuzoj/internal/problem/service"
	pkgerrors "fuzoj/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProblemRPCServer implements gRPC problem service.
type ProblemRPCServer struct {
	problemv1.UnimplementedProblemServiceServer
	service *service.ProblemService
}

// NewProblemRPCServer creates a new gRPC server.
func NewProblemRPCServer(svc *service.ProblemService) *ProblemRPCServer {
	return &ProblemRPCServer{service: svc}
}

// RegisterProblemService registers the gRPC server.
func RegisterProblemService(grpcServer *grpc.Server, svc *service.ProblemService) {
	problemv1.RegisterProblemServiceServer(grpcServer, NewProblemRPCServer(svc))
}

// GetLatest returns latest published meta.
func (s *ProblemRPCServer) GetLatest(ctx context.Context, req *problemv1.GetLatestRequest) (*problemv1.GetLatestResponse, error) {
	if req == nil || req.GetProblemId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "problem_id is required")
	}

	meta, err := s.service.GetLatestMeta(ctx, req.GetProblemId())
	if err != nil {
		return nil, mapError(err)
	}

	return &problemv1.GetLatestResponse{
		Meta: &problemv1.ProblemLatestMeta{
			ProblemId:    meta.ProblemID,
			Version:      int32(meta.Version),
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
