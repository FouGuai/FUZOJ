// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"

	"fuzoj/pkg/bootstrap"
	"fuzoj/services/submit_service/internal/config"
	"fuzoj/services/submit_service/internal/handler"
	"fuzoj/services/submit_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/submit.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	boot := c.Bootstrap
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
	c = full

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
	handler.RegisterHandlers(server, ctx)

	if ctx.StatusFinalQueue != nil {
		go ctx.StatusFinalQueue.Start()
		defer ctx.StatusFinalQueue.Stop()
	}
	if ctx.TopicPushers.Level0 != nil {
		defer ctx.TopicPushers.Level0.Close()
	}
	if ctx.TopicPushers.Level1 != nil {
		defer ctx.TopicPushers.Level1.Close()
	}
	if ctx.TopicPushers.Level2 != nil {
		defer ctx.TopicPushers.Level2.Close()
	}
	if ctx.TopicPushers.Level3 != nil {
		defer ctx.TopicPushers.Level3.Close()
	}

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
