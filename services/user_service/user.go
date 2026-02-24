// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"

	"fuzoj/pkg/bootstrap"
	"fuzoj/services/user_service/internal/config"
	"fuzoj/services/user_service/internal/handler"
	"fuzoj/services/user_service/internal/logic/auth_app"
	"fuzoj/services/user_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/user.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	runtime, err := bootstrap.LoadRestRuntime(context.Background(), c.Bootstrap)
	if err != nil {
		logx.Errorf("load runtime config failed: %v", err)
		return
	}
	if err := bootstrap.ApplyRestRuntime(&c.RestConf, runtime); err != nil {
		logx.Errorf("apply runtime config failed: %v", err)
		return
	}

	var logConf logx.LogConf
	if err := bootstrap.LoadJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.Log, &logConf); err != nil {
		logx.Errorf("load log config failed: %v", err)
		return
	}
	logx.MustSetup(logConf)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	registerKey, err := bootstrap.RestRegisterKey(runtime)
	if err != nil {
		logx.Errorf("build register key failed: %v", err)
		return
	}
	registerValue, err := bootstrap.RestRegisterValue(runtime)
	if err != nil {
		logx.Errorf("build register value failed: %v", err)
		return
	}
	pub, err := bootstrap.RegisterService(c.Bootstrap.Etcd, registerKey, registerValue)
	if err != nil {
		logx.Errorf("register service failed: %v", err)
		return
	}
	defer pub.Stop()

	ctx := svc.NewServiceContext(c)
	auth_app.InitAuth(context.Background(), ctx)
	handler.RegisterHandlers(server, ctx)

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
