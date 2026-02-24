package bootstrap

import "github.com/zeromicro/go-zero/core/discov"

// Config holds bootstrap settings for etcd-based runtime and log configs.
type Config struct {
	Etcd discov.EtcdConf `json:"etcd"`
	Keys Keys            `json:"keys"`
}

// Keys defines etcd keys for runtime and log configuration.
type Keys struct {
	Config     string `json:"config"`
	Runtime    string `json:"runtime"`
	RpcRuntime string `json:"rpcRuntime,optional"`
	Log        string `json:"log"`
}
