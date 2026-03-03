package main

import (
	"context"
	"flag"

	rankpb "fuzoj/api/proto/rank"
	"fuzoj/pkg/bootstrap"
	"fuzoj/services/rank_rpc_service/internal/config"
	"fuzoj/services/rank_rpc_service/internal/server"
	"fuzoj/services/rank_rpc_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/rank.yaml", "the config file")

func main() {
	flag.Parse()

	var bootCfg struct {
		Bootstrap bootstrap.Config `json:"bootstrap"`
	}
	conf.MustLoad(*configFile, &bootCfg)

	boot := bootCfg.Bootstrap
	if boot.Keys.Config == "" {
		logx.Error("bootstrap.keys.config is required")
		return
	}

	var full config.Config
	if err := bootstrap.LoadConfig(context.Background(), boot.Etcd, boot.Keys.Config, &full); err != nil {
		logx.Errorf("load full config failed: %v", err)
		return
	}
	full.Bootstrap = boot
	c := full

	runtime, err := bootstrap.LoadRpcRuntime(context.Background(), c.Bootstrap)
	if err != nil {
		logx.Errorf("load rpc runtime config failed: %v", err)
		return
	}
	changed, err := bootstrap.AssignRandomRpcListenOn(&runtime)
	if err != nil {
		logx.Errorf("assign random rpc listenOn failed: %v", err)
		return
	}
	if changed {
		if err := bootstrap.PutJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.RpcRuntime, runtime); err != nil {
			logx.Errorf("update rpc runtime config failed: %v", err)
			return
		}
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
		rankpb.RegisterRankRpcServer(grpcServer, server.NewRankRpcServer(ctx))
		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	logx.Infof("Starting rpc server at %s...", c.ListenOn)
	s.Start()
}
