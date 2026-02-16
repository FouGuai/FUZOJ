package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResponseInfo carries response details.
type ResponseInfo struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
}

// Client wraps HTTP requests for CLI.
type Client struct {
	baseURL       string
	timeout       time.Duration
	tokenProvider func() string
}

func New(baseURL string, timeout time.Duration, tokenProvider func() string) *Client {
	return &Client{
		baseURL:       baseURL,
		timeout:       timeout,
		tokenProvider: tokenProvider,
	}
}

func (c *Client) SetBaseURL(baseURL string) {
	c.baseURL = baseURL
}

func (c *Client) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		c.timeout = timeout
	}
}

func (c *Client) Do(ctx context.Context, method, path string, headers map[string]string, body []byte) (ResponseInfo, error) {
	var info ResponseInfo
	client := &http.Client{Timeout: c.timeout}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", c.baseURL, path), reader)
	if err != nil {
		return info, fmt.Errorf("build request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		if v != "" {
			req.Header.Set(k, v)
		}
	}
	if c.tokenProvider != nil {
		if token := c.tokenProvider(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	info.Duration = time.Since(start)
	if err != nil {
		return info, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	info.StatusCode = resp.StatusCode
	info.Headers = resp.Header
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return info, fmt.Errorf("read response body failed: %w", err)
	}
	info.Body = bodyBytes
	return info, nil
}
