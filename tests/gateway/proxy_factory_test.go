package gateway_test

import (
	"net/url"
	"testing"
	"time"

	"fuzoj/internal/gateway/service"
)

func TestProxyFactoryGet(t *testing.T) {
	upstreamURL, err := url.Parse("http://127.0.0.1:8081")
	if err != nil {
		t.Fatalf("parse url failed: %v", err)
	}
	factory := service.NewProxyFactory(service.ProxyConfig{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       time.Second,
		ResponseHeaderTimeout: time.Second,
		TLSHandshakeTimeout:   time.Second,
		DialTimeout:           time.Second,
	}, map[string]*url.URL{"api": upstreamURL})

	proxyA, err := factory.Get("api")
	if err != nil {
		t.Fatalf("get proxy failed: %v", err)
	}
	proxyB, err := factory.Get("api")
	if err != nil {
		t.Fatalf("get proxy failed: %v", err)
	}
	if proxyA != proxyB {
		t.Fatalf("expected cached proxy instance")
	}
}

func TestProxyFactoryMissingUpstream(t *testing.T) {
	factory := service.NewProxyFactory(service.ProxyConfig{}, map[string]*url.URL{})
	if _, err := factory.Get("missing"); err == nil {
		t.Fatalf("expected error for missing upstream")
	}
}
