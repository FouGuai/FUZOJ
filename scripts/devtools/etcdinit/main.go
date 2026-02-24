package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

type EtcdConf struct {
	Hosts              []string `yaml:"hosts"`
	User               string   `yaml:"user"`
	Pass               string   `yaml:"pass"`
	CertFile           string   `yaml:"certFile"`
	CertKeyFile        string   `yaml:"certKeyFile"`
	CACertFile         string   `yaml:"caCertFile"`
	InsecureSkipVerify bool     `yaml:"insecureSkipVerify"`
}

type BootstrapKeys struct {
	Config     string `yaml:"config"`
	Runtime    string `yaml:"runtime"`
	RpcRuntime string `yaml:"rpcRuntime"`
	Log        string `yaml:"log"`
}

type Bootstrap struct {
	Etcd EtcdConf      `yaml:"etcd"`
	Keys BootstrapKeys `yaml:"keys"`
}

type RpcConf struct {
	ListenOn string `yaml:"ListenOn"`
	Etcd     struct {
		Hosts []string `yaml:"Hosts"`
		Key   string   `yaml:"Key"`
	} `yaml:"Etcd"`
}

type ServiceMeta struct {
	Name        string          `yaml:"Name"`
	Host        string          `yaml:"Host"`
	Port        int             `yaml:"Port"`
	Timeout     interface{}     `yaml:"Timeout"`
	Middlewares map[string]bool `yaml:"Middlewares"`
	Bootstrap   Bootstrap       `yaml:"bootstrap"`
	Rpc         *RpcConf        `yaml:"Rpc"`
	Logger      map[string]any  `yaml:"logger"`
}

type ServiceSpec struct {
	Name string
	Path string
}

type EtcdWriter struct {
	cli *clientv3.Client
}

func main() {
	var only string
	var configDir string
	var dryRun bool
	flag.StringVar(&only, "only", "", "Comma-separated service list")
	flag.StringVar(&configDir, "config-dir", "services", "Service config root")
	flag.BoolVar(&dryRun, "dry-run", false, "Print etcd writes without executing")
	flag.Parse()

	serviceSpecs := []ServiceSpec{
		{Name: "gateway", Path: "gateway_service/etc/gateway.yaml"},
		{Name: "user", Path: "user_service/etc/user.yaml"},
		{Name: "problem", Path: "problem_service/etc/problem.yaml"},
		{Name: "submit", Path: "submit_service/etc/submit.yaml"},
		{Name: "judge", Path: "judge_service/etc/judge.yaml"},
	}

	onlySet := map[string]bool{}
	if strings.TrimSpace(only) != "" {
		for _, item := range strings.Split(only, ",") {
			name := strings.TrimSpace(item)
			if name != "" {
				onlySet[name] = true
			}
		}
	}

	for _, spec := range serviceSpecs {
		if len(onlySet) > 0 && !onlySet[spec.Name] {
			continue
		}
		servicePath := filepath.Join(configDir, spec.Path)
		raw, meta, err := loadServiceConfig(servicePath)
		if err != nil {
			fail(err)
		}

		if meta.Name == "" {
			meta.Name = spec.Name
		}
		keys := defaultKeys(meta.Name, meta.Bootstrap.Keys)

		if len(meta.Bootstrap.Etcd.Hosts) == 0 {
			fail(fmt.Errorf("bootstrap.etcd.hosts is required for %s", meta.Name))
		}

		cli, err := newEtcdClient(meta.Bootstrap.Etcd)
		if err != nil {
			fail(fmt.Errorf("init etcd client failed for %s: %w", meta.Name, err))
		}
		writer := &EtcdWriter{cli: cli}

		configPayload := removeBootstrap(raw)
		if err := writer.putJSON(keys.Config, configPayload, dryRun); err != nil {
			fail(err)
		}

		runtimePayload, err := buildRestRuntime(meta)
		if err != nil {
			fail(err)
		}
		if err := writer.putJSON(keys.Runtime, runtimePayload, dryRun); err != nil {
			fail(err)
		}

		if meta.Rpc != nil && meta.Rpc.ListenOn != "" {
			rpcPayload, err := buildRpcRuntime(meta)
			if err != nil {
				fail(err)
			}
			if err := writer.putJSON(keys.RpcRuntime, rpcPayload, dryRun); err != nil {
				fail(err)
			}
		}

		logPayload := buildLogConfig(meta, raw)
		if err := writer.putJSON(keys.Log, logPayload, dryRun); err != nil {
			fail(err)
		}

		_ = cli.Close()
	}
}

func loadServiceConfig(path string) (map[string]any, ServiceMeta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ServiceMeta{}, fmt.Errorf("read config failed: %w", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, ServiceMeta{}, fmt.Errorf("parse yaml failed: %w", err)
	}
	normalized := normalizeValue(raw)
	rawMap, ok := normalized.(map[string]any)
	if !ok {
		return nil, ServiceMeta{}, errors.New("config root must be a map")
	}

	var meta ServiceMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, ServiceMeta{}, fmt.Errorf("parse meta failed: %w", err)
	}

	return rawMap, meta, nil
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			out[normalizeKey(key)] = normalizeValue(child)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			strKey, ok := key.(string)
			if !ok {
				continue
			}
			out[normalizeKey(strKey)] = normalizeValue(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeValue(item))
		}
		return out
	default:
		return value
	}
}

func normalizeKey(key string) string {
	if key == "" {
		return key
	}
	if key == "MinIO" {
		return "minio"
	}
	if key == "RPC" {
		return "rpc"
	}
	if key == "URL" {
		return "url"
	}
	if key == "ID" {
		return "id"
	}
	r := []rune(key)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func removeBootstrap(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		if key == "bootstrap" || key == "Bootstrap" {
			continue
		}
		out[key] = value
	}
	return out
}

func defaultKeys(service string, provided BootstrapKeys) BootstrapKeys {
	keys := provided
	if keys.Config == "" {
		keys.Config = service + ".config"
	}
	if keys.Runtime == "" {
		keys.Runtime = service + ".rest.runtime"
	}
	if keys.RpcRuntime == "" {
		keys.RpcRuntime = service + ".rpc.runtime"
	}
	if keys.Log == "" {
		keys.Log = service + ".log"
	}
	return keys
}

func buildRestRuntime(meta ServiceMeta) (map[string]any, error) {
	if meta.Name == "" || meta.Host == "" || meta.Port <= 0 {
		return nil, fmt.Errorf("missing name/host/port in config for %s", meta.Name)
	}
	runtime := map[string]any{
		"name": meta.Name,
		"host": meta.Host,
		"port": meta.Port,
	}
	if timeout, ok := normalizeDuration(meta.Timeout); ok {
		runtime["timeout"] = timeout
	}
	if len(meta.Middlewares) > 0 {
		runtime["middlewares"] = meta.Middlewares
	}
	return runtime, nil
}

func buildRpcRuntime(meta ServiceMeta) (map[string]any, error) {
	if meta.Rpc == nil {
		return nil, errors.New("rpc config missing")
	}
	if meta.Rpc.ListenOn == "" {
		return nil, errors.New("rpc listenOn is required")
	}
	if len(meta.Rpc.Etcd.Hosts) == 0 || meta.Rpc.Etcd.Key == "" {
		return nil, errors.New("rpc etcd hosts/key are required")
	}
	return map[string]any{
		"listenOn": meta.Rpc.ListenOn,
		"etcd": map[string]any{
			"hosts": meta.Rpc.Etcd.Hosts,
			"key":   meta.Rpc.Etcd.Key,
		},
	}, nil
}

func buildLogConfig(meta ServiceMeta, raw map[string]any) map[string]any {
	if logger, ok := raw["logger"].(map[string]any); ok && len(logger) > 0 {
		return logger
	}
	return map[string]any{
		"serviceName": meta.Name,
		"mode":        "console",
		"encoding":    "json",
		"level":       "info",
	}
}

func normalizeDuration(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", false
	case string:
		if strings.TrimSpace(v) == "" {
			return "", false
		}
		return v, true
	case int:
		if v == 0 {
			return "0s", true
		}
		return fmt.Sprintf("%ds", v), true
	case int64:
		if v == 0 {
			return "0s", true
		}
		return fmt.Sprintf("%ds", v), true
	case float64:
		if v == 0 {
			return "0s", true
		}
		return fmt.Sprintf("%gs", v), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

func (w *EtcdWriter) putJSON(key string, payload map[string]any, dryRun bool) error {
	if key == "" {
		return errors.New("etcd key is required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal %s failed: %w", key, err)
	}
	if dryRun {
		fmt.Printf("%s => %s\n", key, string(data))
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = w.cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("put %s failed: %w", key, err)
	}
	fmt.Printf("written %s\n", key)
	return nil
}

func newEtcdClient(conf EtcdConf) (*clientv3.Client, error) {
	tlsConfig, err := buildTLSConfig(conf)
	if err != nil {
		return nil, err
	}
	cfg := clientv3.Config{
		Endpoints:   conf.Hosts,
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	}
	if conf.User != "" {
		cfg.Username = conf.User
		cfg.Password = conf.Pass
	}
	return clientv3.New(cfg)
}

func buildTLSConfig(conf EtcdConf) (*tls.Config, error) {
	if conf.CertFile == "" && conf.CertKeyFile == "" && conf.CACertFile == "" {
		return nil, nil
	}
	if conf.CertFile == "" || conf.CertKeyFile == "" || conf.CACertFile == "" {
		return nil, errors.New("certFile, certKeyFile, and caCertFile must be set together")
	}
	cert, err := tls.LoadX509KeyPair(conf.CertFile, conf.CertKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load cert failed: %w", err)
	}
	caData, err := os.ReadFile(conf.CACertFile)
	if err != nil {
		return nil, fmt.Errorf("read ca cert failed: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caData) {
		return nil, errors.New("append ca cert failed")
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		InsecureSkipVerify: conf.InsecureSkipVerify,
	}, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
