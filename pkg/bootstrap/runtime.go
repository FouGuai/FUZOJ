package bootstrap

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/zeromicro/go-zero/core/discov"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

// RestRuntime holds runtime config and register key for REST service.
type RestRuntime struct {
	Name        string              `json:"name"`
	Host        string              `json:"host"`
	Port        int                 `json:"port"`
	Timeout     int64               `json:"timeout,optional"`
	Middlewares rest.MiddlewaresConf `json:"middlewares,optional"`
	RegisterKey string `json:"registerKey,optional"`
}

// RpcRuntime holds runtime config for RPC service.
type RpcRuntime struct {
	ListenOn string         `json:"listenOn"`
	Etcd     discov.EtcdConf `json:"etcd"`
}

// LoadRestRuntime loads REST runtime config from etcd.
func LoadRestRuntime(ctx context.Context, boot Config) (RestRuntime, error) {
	if boot.Keys.Runtime == "" {
		return RestRuntime{}, fmt.Errorf("bootstrap.keys.runtime is required")
	}
	var runtime RestRuntime
	if err := LoadJSON(ctx, boot.Etcd, boot.Keys.Runtime, &runtime); err != nil {
		return RestRuntime{}, err
	}
	return runtime, nil
}

// LoadRpcRuntime loads RPC runtime config from etcd.
func LoadRpcRuntime(ctx context.Context, boot Config) (RpcRuntime, error) {
	if boot.Keys.RpcRuntime == "" {
		return RpcRuntime{}, fmt.Errorf("bootstrap.keys.rpcRuntime is required")
	}
	var runtime RpcRuntime
	if err := LoadJSON(ctx, boot.Etcd, boot.Keys.RpcRuntime, &runtime); err != nil {
		return RpcRuntime{}, err
	}
	return runtime, nil
}

// ApplyRestRuntime overrides rest config with runtime values.
func ApplyRestRuntime(target *rest.RestConf, runtime RestRuntime) error {
	if target == nil {
		return fmt.Errorf("target rest config is required")
	}
	if runtime.Name == "" {
		return fmt.Errorf("runtime name is required")
	}
	if runtime.Host == "" {
		return fmt.Errorf("runtime host is required")
	}
	if runtime.Port <= 0 {
		return fmt.Errorf("runtime port is required")
	}
	target.Name = runtime.Name
	target.Host = runtime.Host
	target.Port = runtime.Port
	target.Timeout = runtime.Timeout
	target.Middlewares = runtime.Middlewares
	return nil
}

// ApplyRpcRuntime overrides rpc config with runtime values.
func ApplyRpcRuntime(target *zrpc.RpcServerConf, runtime RpcRuntime) error {
	if target == nil {
		return fmt.Errorf("target rpc config is required")
	}
	if runtime.ListenOn == "" {
		return fmt.Errorf("runtime listenOn is required")
	}
	target.ListenOn = runtime.ListenOn
	target.Etcd = runtime.Etcd
	return nil
}

// RestRegisterValue returns host:port for registration.
func RestRegisterValue(runtime RestRuntime) (string, error) {
	if runtime.Host == "" || runtime.Port <= 0 {
		return "", fmt.Errorf("invalid runtime host or port")
	}
	return net.JoinHostPort(runtime.Host, strconv.Itoa(runtime.Port)), nil
}

// RestRegisterKey returns register key, defaulting to name.rest.
func RestRegisterKey(runtime RestRuntime) (string, error) {
	if runtime.RegisterKey != "" {
		return runtime.RegisterKey, nil
	}
	if runtime.Name == "" {
		return "", fmt.Errorf("runtime name is required")
	}
	return runtime.Name + ".rest", nil
}

// AssignRandomRestPort assigns a random port if runtime.Port is zero.
func AssignRandomRestPort(runtime *RestRuntime) (bool, error) {
	if runtime == nil {
		return false, fmt.Errorf("runtime is required")
	}
	if runtime.Port > 0 {
		return false, nil
	}
	if runtime.Host == "" {
		runtime.Host = "0.0.0.0"
	}
	port, err := randomPort(runtime.Host)
	if err != nil {
		return false, err
	}
	runtime.Port = port
	return true, nil
}

// AssignRandomRpcListenOn assigns a random port if listenOn ends with :0.
func AssignRandomRpcListenOn(runtime *RpcRuntime) (bool, error) {
	if runtime == nil {
		return false, fmt.Errorf("runtime is required")
	}
	host, port, err := net.SplitHostPort(runtime.ListenOn)
	if err != nil {
		return false, fmt.Errorf("invalid listenOn: %w", err)
	}
	if port != "0" {
		return false, nil
	}
	if host == "" {
		host = "0.0.0.0"
	}
	randomPort, err := randomPort(host)
	if err != nil {
		return false, err
	}
	runtime.ListenOn = net.JoinHostPort(host, strconv.Itoa(randomPort))
	return true, nil
}

func randomPort(host string) (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, fmt.Errorf("listen on random port failed: %w", err)
	}
	defer func() { _ = listener.Close() }()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected listener addr type: %T", listener.Addr())
	}
	return addr.Port, nil
}
