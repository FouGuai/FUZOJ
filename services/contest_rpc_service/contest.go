package main

import (
	"context"
	"flag"

	contestpb "fuzoj/api/proto/contest"
	"fuzoj/pkg/bootstrap"
	"fuzoj/services/contest_rpc_service/internal/config"
	"fuzoj/services/contest_rpc_service/internal/server"
	"fuzoj/services/contest_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/contest.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	runtime, err := bootstrap.LoadRpcRuntime(context.Background(), c.Bootstrap)
	if err != nil {
		logx.Errorf("load rpc runtime config failed: %v", err)
		return
	}
	if err := bootstrap.ApplyRpcRuntime(&c.RpcServerConf, runtime); err != nil {
		logx.Errorf("apply rpc runtime config failed: %v", err)
		return
	}

	var logConf logx.LogConf
	if err := bootstrap.LoadJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.Log, &logConf); err != nil {
		logx.Errorf("load log config failed: %v", err)
		return
	}
	logx.MustSetup(logConf)

	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		contestpb.RegisterContestRpcServer(grpcServer, server.NewContestRpcServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	logx.Infof("Starting rpc server at %s...", c.ListenOn)
	s.Start()
}
