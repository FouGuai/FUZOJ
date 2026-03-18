package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fuzoj/pkg/errors"
	"fuzoj/services/gateway_service/internal/config"
	"fuzoj/services/gateway_service/internal/discovery"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest/httpc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// NewHTTPForwarder builds a handler that forwards to targets selected by picker.
func NewHTTPForwarder(picker discovery.Picker, target config.HttpClientConf) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetAddr, err := picker.Pick()
		if err != nil {
			logx.WithContext(r.Context()).Errorf("no upstream targets available: %v", err)
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
			logx.WithContext(r.Context()).Errorf("forward request failed: %v", err)
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
		if isStreamResponse(resp) {
			if err = copyStreamResponse(w, resp.Body); err != nil {
				logx.WithContext(r.Context()).Errorf("copy upstream stream response failed: %v", err)
			}
			return
		}
		if _, err = io.Copy(w, resp.Body); err != nil {
			logx.WithContext(r.Context()).Errorf("copy upstream response failed: %v", err)
		}
	}
}

func isStreamResponse(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	return strings.Contains(contentType, "text/event-stream")
}

func copyStreamResponse(w http.ResponseWriter, body io.Reader) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		_, err := io.Copy(w, body)
		return err
	}

	buf := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return readErr
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
