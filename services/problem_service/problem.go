// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"

	problemv1 "fuzoj/api/gen/problem/v1"
	"fuzoj/services/problem_service/internal/config"
	"fuzoj/services/problem_service/internal/grpcserver"
	"fuzoj/services/problem_service/internal/handler"
	"fuzoj/services/problem_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
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

	var rpcServer *zrpc.RpcServer
	if c.Rpc.ListenOn != "" {
		rpcServer = zrpc.MustNewServer(c.Rpc, func(grpcServer *grpc.Server) {
			problemv1.RegisterProblemServiceServer(grpcServer, grpcserver.NewProblemRPCServer(ctx))
		})
		go func() {
			logx.Infof("Starting RPC server at %s...", c.Rpc.ListenOn)
			rpcServer.Start()
		}()
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
	if rpcServer != nil {
		defer rpcServer.Stop()
	}

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
