// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"net"

	problemv1 "fuzoj/api/gen/problem/v1"
	"fuzoj/services/problem_service/internal/config"
	"fuzoj/services/problem_service/internal/grpcserver"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"google.golang.org/grpc"
)

var configFile = flag.String("f", "etc/problem.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	var grpcServer *grpc.Server
	if c.Grpc.Addr != "" {
		lis, err := net.Listen("tcp", c.Grpc.Addr)
		if err != nil {
			logx.Errorf("Start grpc listener failed: %v", err)
		} else {
			grpcServer = grpc.NewServer()
			problemv1.RegisterProblemServiceServer(grpcServer, grpcserver.NewProblemRPCServer(ctx))
			go func() {
				logx.Infof("Starting gRPC server at %s...", c.Grpc.Addr)
				if err := grpcServer.Serve(lis); err != nil {
					logx.Errorf("gRPC server stopped: %v", err)
				}
			}()
		}
	}

	if ctx.CleanupQueue != nil {
		go ctx.CleanupQueue.Start()
		defer ctx.CleanupQueue.Stop()
	}
	if ctx.CleanupPublisher != nil {
		defer ctx.CleanupPublisher.Close()
	}
	if ctx.DeadLetterPusher != nil {
		defer ctx.DeadLetterPusher.Close()
	}
	if grpcServer != nil {
		defer grpcServer.GracefulStop()
	}

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
