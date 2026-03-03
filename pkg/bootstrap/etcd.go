package bootstrap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/discov"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	defaultDialTimeout = 3 * time.Second
	defaultReqTimeout  = 3 * time.Second
)

// LoadJSON loads JSON value from etcd into out.
func LoadJSON(ctx context.Context, etcdConf discov.EtcdConf, key string, out any) error {
	if err := etcdConf.Validate(); err != nil {
		return fmt.Errorf("invalid etcd config: %w", err)
	}
	if key == "" {
		return fmt.Errorf("etcd key is required")
	}
	cli, err := newEtcdClient(etcdConf)
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	reqCtx, cancel := context.WithTimeout(ctx, defaultReqTimeout)
	defer cancel()

	resp, err := cli.Get(reqCtx, key)
	if err != nil {
		return fmt.Errorf("get etcd key %s failed: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return fmt.Errorf("etcd key %s not found", key)
	}

	if err := unmarshalConfig(resp.Kvs[0].Value, out); err != nil {
		return fmt.Errorf("unmarshal etcd key %s failed: %w", key, err)
	}
	return nil
}

// LoadConfig loads full service config JSON from etcd.
func LoadConfig(ctx context.Context, etcdConf discov.EtcdConf, key string, out any) error {
	if key == "" {
		return fmt.Errorf("config key is required")
	}
	return LoadJSON(ctx, etcdConf, key, out)
}

// PutJSON writes JSON value to etcd key.
func PutJSON(ctx context.Context, etcdConf discov.EtcdConf, key string, value any) error {
	if err := etcdConf.Validate(); err != nil {
		return fmt.Errorf("invalid etcd config: %w", err)
	}
	if key == "" {
		return fmt.Errorf("etcd key is required")
	}
	cli, err := newEtcdClient(etcdConf)
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal etcd key %s failed: %w", key, err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, defaultReqTimeout)
	defer cancel()

	if _, err := cli.Put(reqCtx, key, string(data)); err != nil {
		return fmt.Errorf("put etcd key %s failed: %w", key, err)
	}
	return nil
}

// RegisterService registers value under key with lease keepalive.
func RegisterService(etcdConf discov.EtcdConf, key, value string) (*discov.Publisher, error) {
	if err := etcdConf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid etcd config: %w", err)
	}
	if key == "" {
		return nil, fmt.Errorf("register key is required")
	}
	if value == "" {
		return nil, fmt.Errorf("register value is required")
	}

	opts := make([]discov.PubOption, 0, 3)
	if etcdConf.HasAccount() {
		opts = append(opts, discov.WithPubEtcdAccount(etcdConf.User, etcdConf.Pass))
	}
	if etcdConf.HasTLS() {
		opts = append(opts, discov.WithPubEtcdTLS(etcdConf.CertFile, etcdConf.CertKeyFile, etcdConf.CACertFile, etcdConf.InsecureSkipVerify))
	}
	if etcdConf.HasID() {
		opts = append(opts, discov.WithId(etcdConf.ID))
	}

	pub := discov.NewPublisher(etcdConf.Hosts, key, value, opts...)
	if err := pub.KeepAlive(); err != nil {
		return nil, fmt.Errorf("register service failed: %w", err)
	}
	return pub, nil
}

func newEtcdClient(etcdConf discov.EtcdConf) (*clientv3.Client, error) {
	cfg := clientv3.Config{
		Endpoints:   etcdConf.Hosts,
		DialTimeout: defaultDialTimeout,
	}
	if etcdConf.HasAccount() {
		cfg.Username = etcdConf.User
		cfg.Password = etcdConf.Pass
	}
	if etcdConf.HasTLS() {
		tlsCfg, err := buildTLSConfig(etcdConf)
		if err != nil {
			return nil, err
		}
		cfg.TLS = tlsCfg
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("create etcd client failed: %w", err)
	}
	return cli, nil
}

func unmarshalConfig(data []byte, out any) error {
	if out == nil {
		return fmt.Errorf("output target is required")
	}
	value := reflect.ValueOf(out)
	if value.Kind() == reflect.Pointer && value.Elem().Kind() == reflect.Struct {
		return conf.LoadFromJsonBytes(data, out)
	}
	return json.Unmarshal(data, out)
}

// DecodeJSON decodes JSON bytes into output.
func DecodeJSON(data []byte, out any) error {
	return unmarshalConfig(data, out)
}

// StartWatchJSON watches etcd key and invokes callback on updates.
func StartWatchJSON(ctx context.Context, etcdConf discov.EtcdConf, key string, onChange func([]byte)) (context.CancelFunc, error) {
	if err := etcdConf.Validate(); err != nil {
		return nil, fmt.Errorf("invalid etcd config: %w", err)
	}
	if key == "" {
		return nil, fmt.Errorf("etcd key is required")
	}
	if onChange == nil {
		return nil, fmt.Errorf("onChange is required")
	}
	cli, err := newEtcdClient(etcdConf)
	if err != nil {
		return nil, err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	go func() {
		defer func() { _ = cli.Close() }()
		ch := cli.Watch(watchCtx, key)
		for resp := range ch {
			if resp.Canceled {
				return
			}
			for _, ev := range resp.Events {
				if ev.Type != clientv3.EventTypePut {
					continue
				}
				if ev.Kv == nil || len(ev.Kv.Value) == 0 {
					continue
				}
				onChange(ev.Kv.Value)
			}
		}
	}()
	return cancel, nil
}

func buildTLSConfig(etcdConf discov.EtcdConf) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(etcdConf.CertFile, etcdConf.CertKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load etcd cert failed: %w", err)
	}
	caBytes, err := os.ReadFile(etcdConf.CACertFile)
	if err != nil {
		return nil, fmt.Errorf("read etcd ca file failed: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("append etcd ca failed")
	}
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            pool,
		InsecureSkipVerify: etcdConf.InsecureSkipVerify,
	}, nil
}
