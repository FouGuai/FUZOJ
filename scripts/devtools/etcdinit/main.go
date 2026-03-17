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
	"strconv"
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
	Switch     string `yaml:"switch"`
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
	flag.StringVar(&configDir, "config-dir", "configs/etcdinit", "Service config root")
	flag.BoolVar(&dryRun, "dry-run", false, "Print etcd writes without executing")
	flag.Parse()

	baseRoot := findRepoRoot()
	if !filepath.IsAbs(configDir) {
		configDir = filepath.Join(baseRoot, configDir)
	}

	serviceSpecs := []ServiceSpec{
		{Name: "gateway", Path: "gateway_service/etc/gateway.yaml"},
		{Name: "user", Path: "user_service/etc/user.yaml"},
		{Name: "problem", Path: "problem_service/etc/problem.yaml"},
		{Name: "submit", Path: "submit_service/etc/submit.yaml"},
		{Name: "status", Path: "status_service/etc/status.yaml"},
		{Name: "judge", Path: "judge_service/etc/judge.yaml"},
		{Name: "contest", Path: "contest_service/etc/contest.yaml"},
		{Name: "contest.rpc", Path: "contest_rpc_service/etc/contest.yaml"},
		{Name: "rank", Path: "rank_service/etc/rank.yaml"},
		{Name: "status-sse", Path: "status_sse_service/etc/status_sse.yaml"},
		{Name: "rank-ws", Path: "rank_ws_service/etc/rank_ws.yaml"},
		{Name: "rank-rpc", Path: "rank_rpc_service/etc/rank.yaml"},
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

		if meta.Host != "" && meta.Port >= 0 {
			runtimePayload, err := buildRestRuntime(meta)
			if err != nil {
				fail(err)
			}
			if err := writer.putJSON(keys.Runtime, runtimePayload, dryRun); err != nil {
				fail(err)
			}
		}

		if meta.Rpc != nil || raw["listenOn"] != nil {
			rpcPayload, err := buildRpcRuntime(meta, raw)
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

			if keys.Switch != "" {
				if payload, ok := raw["switch"]; ok {
					switchPayload, ok := payload.(map[string]any)
					if !ok {
						fail(fmt.Errorf("switch config must be an object, got %T", payload))
					}
					if err := writer.putJSON(keys.Switch, switchPayload, dryRun); err != nil {
						fail(err)
					}
				}
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
		if meta.Name == "" || meta.Host == "" || meta.Port < 0 {
			return nil, fmt.Errorf("missing name/host/port in config for %s", meta.Name)
		}
	}
	runtime := map[string]any{
		"name": meta.Name,
		"host": meta.Host,
		"port": meta.Port,
	}
	if timeout, ok, err := normalizeRestTimeout(meta.Timeout); err != nil {
		return nil, err
	} else if ok {
		runtime["timeout"] = timeout
	}
	if len(meta.Middlewares) > 0 {
		runtime["middlewares"] = meta.Middlewares
	}
	return runtime, nil
}

func buildRpcRuntime(meta ServiceMeta, raw map[string]any) (map[string]any, error) {
	listenOn := ""
	etcdHosts := []string{}
	etcdKey := ""

	if meta.Rpc != nil {
		listenOn = meta.Rpc.ListenOn
		etcdHosts = meta.Rpc.Etcd.Hosts
		etcdKey = meta.Rpc.Etcd.Key
	} else {
		if v, ok := raw["listenOn"].(string); ok {
			listenOn = v
		}
		if etcdRaw, ok := raw["etcd"].(map[string]any); ok {
			etcdHosts = parseStringSlice(etcdRaw["hosts"])
			if key, ok := etcdRaw["key"].(string); ok {
				etcdKey = key
			}
		}
	}

	if listenOn == "" {
		return nil, errors.New("rpc listenOn is required")
	}
	if len(etcdHosts) == 0 || etcdKey == "" {
		return nil, errors.New("rpc etcd hosts/key are required")
	}
	return map[string]any{
		"name": meta.Name,
		"listenOn": listenOn,
		"etcd": map[string]any{
			"hosts": etcdHosts,
			"key":   etcdKey,
		},
	}, nil
}

func parseStringSlice(value any) []string {
	out := []string{}
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
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

func normalizeRestTimeout(value any) (int64, bool, error) {
	switch v := value.(type) {
	case nil:
		return 0, false, nil
	case int:
		if v <= 0 {
			return 0, false, nil
		}
		return int64(v), true, nil
	case int64:
		if v <= 0 {
			return 0, false, nil
		}
		return v, true, nil
	case float64:
		if v <= 0 {
			return 0, false, nil
		}
		return int64(v), true, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false, nil
		}
		if dur, err := time.ParseDuration(v); err == nil {
			return int64(dur / time.Millisecond), true, nil
		}
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			if ms <= 0 {
				return 0, false, nil
			}
			return ms, true, nil
		}
		return 0, false, fmt.Errorf("invalid rest timeout: %v", v)
	default:
		return 0, false, fmt.Errorf("invalid rest timeout type: %T", value)
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

func findRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	current := filepath.Clean(cwd)
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return cwd
		}
		current = parent
	}
}
