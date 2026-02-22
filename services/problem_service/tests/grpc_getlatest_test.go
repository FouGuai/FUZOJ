package tests

import (
	"context"
	"net"
	"testing"
	"time"

	problemv1 "fuzoj/api/gen/problem/v1"
	pkgerrors "fuzoj/pkg/errors"
	"fuzoj/services/problem_service/internal/grpcserver"
	"fuzoj/services/problem_service/internal/repository"
	"fuzoj/services/problem_service/internal/svc"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const grpcBufSize = 1024 * 1024

func TestGrpcGetLatest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{
					ProblemID:    problemID,
					Version:      1,
					ManifestHash: "mh",
					DataPackKey:  "key",
					DataPackHash: "dh",
					UpdatedAt:    time.Unix(100, 0),
				}, nil
			},
		}
		svcCtx := newTestServiceContext(repo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())

		conn := startGrpcServer(t, svcCtx)
		defer conn.Close()

		client := problemv1.NewProblemServiceClient(conn)
		resp, err := client.GetLatest(context.Background(), &problemv1.GetLatestRequest{ProblemId: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Meta == nil || resp.Meta.ProblemId != 1 {
			t.Fatalf("unexpected response: %+v", resp)
		}
	})

	t.Run("invalid argument", func(t *testing.T) {
		svcCtx := newTestServiceContext(&fakeProblemRepo{}, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		conn := startGrpcServer(t, svcCtx)
		defer conn.Close()

		client := problemv1.NewProblemServiceClient(conn)
		_, err := client.GetLatest(context.Background(), &problemv1.GetLatestRequest{ProblemId: 0})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unexpected code: %v", status.Code(err))
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{}, repository.ErrProblemNotFound
			},
		}
		svcCtx := newTestServiceContext(repo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		conn := startGrpcServer(t, svcCtx)
		defer conn.Close()

		client := problemv1.NewProblemServiceClient(conn)
		_, err := client.GetLatest(context.Background(), &problemv1.GetLatestRequest{ProblemId: 1})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("unexpected code: %v", status.Code(err))
		}
	})

	t.Run("internal error", func(t *testing.T) {
		repo := &fakeProblemRepo{
			getLatestMetaFn: func(ctx context.Context, session sqlx.Session, problemID int64) (repository.ProblemLatestMeta, error) {
				return repository.ProblemLatestMeta{}, pkgerrors.New(pkgerrors.DatabaseError)
			},
		}
		svcCtx := newTestServiceContext(repo, &fakeUploadRepo{}, &fakeStorage{}, defaultTestConfig())
		conn := startGrpcServer(t, svcCtx)
		defer conn.Close()

		client := problemv1.NewProblemServiceClient(conn)
		_, err := client.GetLatest(context.Background(), &problemv1.GetLatestRequest{ProblemId: 1})
		if status.Code(err) != codes.Internal {
			t.Fatalf("unexpected code: %v", status.Code(err))
		}
	})
}

func startGrpcServer(t *testing.T, svcCtx *svc.ServiceContext) *grpc.ClientConn {
	t.Helper()
	listener := bufconn.Listen(grpcBufSize)
	server := grpc.NewServer()
	problemv1.RegisterProblemServiceServer(server, grpcserver.NewProblemRPCServer(svcCtx))
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.GracefulStop()
	})
	dialer := func(ctx context.Context, s string) (net.Conn, error) {
		return listener.Dial()
	}
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn
}
