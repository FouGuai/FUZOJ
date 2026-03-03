// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"context"
	"flag"

	"fuzoj/pkg/bootstrap"
	"fuzoj/services/contest_service/internal/config"
	"fuzoj/services/contest_service/internal/handler"
	"fuzoj/services/contest_service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/contest.yaml", "the config file")

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

	runtime, err := bootstrap.LoadRestRuntime(context.Background(), c.Bootstrap)
	if err != nil {
		logx.Errorf("load runtime config failed: %v", err)
		return
	}
	changed, err := bootstrap.AssignRandomRestPort(&runtime)
	if err != nil {
		logx.Errorf("assign random rest port failed: %v", err)
		return
	}
	if changed {
		if err := bootstrap.PutJSON(context.Background(), c.Bootstrap.Etcd, c.Bootstrap.Keys.Runtime, runtime); err != nil {
			logx.Errorf("update runtime config failed: %v", err)
			return
		}
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

	if ctx.ContestDispatchQueue != nil {
		go ctx.ContestDispatchQueue.Start()
		defer ctx.ContestDispatchQueue.Stop()
	}
	if ctx.JudgePushers.Level0 != nil {
		defer ctx.JudgePushers.Level0.Close()
	}
	if ctx.JudgePushers.Level1 != nil {
		defer ctx.JudgePushers.Level1.Close()
	}
	if ctx.JudgePushers.Level2 != nil {
		defer ctx.JudgePushers.Level2.Close()
	}
	if ctx.JudgePushers.Level3 != nil {
		defer ctx.JudgePushers.Level3.Close()
	}
	if ctx.DeadLetterPusher != nil {
		defer ctx.DeadLetterPusher.Close()
	}

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
