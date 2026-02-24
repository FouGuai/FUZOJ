package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/pkg/utils/logger"
	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/discovery"

	"github.com/zeromicro/go-zero/rest/httpc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"go.uber.org/zap"
)

// NewHTTPForwarder builds a handler that forwards to targets selected by picker.
func NewHTTPForwarder(picker discovery.Picker, target config.HttpClientConf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetAddr, err := picker.Pick()
		if err != nil {
			logger.Error(r.Context(), "no upstream targets available", zap.Error(err))
			httpx.ErrorCtx(r.Context(), w, errors.New(errors.ServiceUnavailable))
			return
		}

		req, err := buildRequestWithTarget(r, targetAddr, target)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		if target.Timeout > 0 {
			timeout := time.Duration(target.Timeout) * time.Millisecond
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			req = req.WithContext(ctx)
		}

		resp, err := httpc.DoRequest(req)
		if err != nil {
			logger.Error(r.Context(), "forward request failed", zap.Error(err))
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		defer resp.Body.Close()

		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.WriteHeader(resp.StatusCode)
		if _, err = io.Copy(w, resp.Body); err != nil {
			logger.Error(r.Context(), "copy upstream response failed", zap.Error(err))
		}
	}
}

func buildRequestWithTarget(r *http.Request, targetAddr string, target config.HttpClientConf) (*http.Request, error) {
	u := *r.URL
	u.Host = targetAddr
	if len(u.Scheme) == 0 {
		u.Scheme = "http"
	}

	if len(target.Prefix) > 0 {
		joined, err := url.JoinPath(target.Prefix, u.Path)
		if err != nil {
			return nil, fmt.Errorf("join path failed: %w", err)
		}
		u.Path = joined
	}

	newReq := &http.Request{
		Method:        r.Method,
		URL:           &u,
		Header:        r.Header.Clone(),
		Proto:         r.Proto,
		ProtoMajor:    r.ProtoMajor,
		ProtoMinor:    r.ProtoMinor,
		ContentLength: r.ContentLength,
		Body:          io.NopCloser(r.Body),
	}
	return newReq.WithContext(r.Context()), nil
}
