package service

import (
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type ProxyConfig struct {
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	IdleConnTimeout       time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	DialTimeout           time.Duration
}

// ProxyFactory builds and caches reverse proxies per upstream.
type ProxyFactory struct {
	config   ProxyConfig
	proxies  map[string]*httputil.ReverseProxy
	mu       sync.RWMutex
	upstream map[string]*url.URL
}

func NewProxyFactory(config ProxyConfig, upstreams map[string]*url.URL) *ProxyFactory {
	return &ProxyFactory{config: config, proxies: make(map[string]*httputil.ReverseProxy), upstream: upstreams}
}

func (f *ProxyFactory) Get(name string) (*httputil.ReverseProxy, error) {
	f.mu.RLock()
	if proxy, ok := f.proxies[name]; ok {
		f.mu.RUnlock()
		return proxy, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()
	if proxy, ok := f.proxies[name]; ok {
		return proxy, nil
	}
	upstream, ok := f.upstream[name]
	if !ok || upstream == nil {
		return nil, errors.New("upstream not found")
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: f.config.DialTimeout, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          f.config.MaxIdleConns,
		MaxIdleConnsPerHost:   f.config.MaxIdleConnsPerHost,
		IdleConnTimeout:       f.config.IdleConnTimeout,
		TLSHandshakeTimeout:   f.config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: f.config.ResponseHeaderTimeout,
	}

	director := func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
	}

	proxy := &httputil.ReverseProxy{
		Director:  director,
		Transport: transport,
	}
	f.proxies[name] = proxy
	return proxy, nil
}
