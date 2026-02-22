// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"

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

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	logx.Infof("Starting server at %s:%d...", c.Host, c.Port)
	server.Start()
}
