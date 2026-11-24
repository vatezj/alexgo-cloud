// Package client 提供 taokouling 的 HTTP 助手与第三方（zhetaoke）口令解析客户端。
package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPHelper 是带超时与有界重试的 HTTP 客户端。
// 重试策略：仅网络错误与 HTTP 5xx 重试；4xx 绝不重试。
type HTTPHelper struct {
	client     *http.Client
	maxRetries int
}

// NewHTTPHelper 构造助手。maxRetries 为「重试次数」（总尝试 = 1 + maxRetries）。
func NewHTTPHelper(timeout time.Duration, maxRetries int) *HTTPHelper {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &HTTPHelper{
		client:     &http.Client{Timeout: timeout},
		maxRetries: maxRetries,
	}
}

// Get 发起 GET 请求，语义见 Do。
func (h *HTTPHelper) Get(ctx context.Context, url string) (status int, body []byte, err error) {
	return h.Do(ctx, http.MethodGet, url, nil, "")
}

// Post 发起带 body 的 POST 请求，语义见 Do。
func (h *HTTPHelper) Post(ctx context.Context, url, contentType string, body []byte) (status int, respBody []byte, err error) {
	return h.Do(ctx, http.MethodPost, url, body, contentType)
}

// Do 执行 HTTP 请求：ctx 贯穿全部尝试；总尝试 = 1 + maxRetries。
// 仅网络错误与 5xx 触发重试（重试间隔 0）；4xx 立即返回不重试。
// 返回最后一次响应的状态码与响应体；err 仅在传输层最终失败（网络错/读错）时非 nil。
// 5xx 重试耗尽后返回末次状态码与响应体、err==nil，由调用方按状态码判失败。
func (h *HTTPHelper) Do(ctx context.Context, method, url string, body []byte, contentType string) (int, []byte, error) {
	attempts := h.maxRetries + 1
	var (
		have5xx bool
		status  int
		last5xx []byte
		lastErr error
	)
	for attempt := 0; attempt < attempts; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, rdr)
		if err != nil {
			return 0, nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := h.client.Do(req)
		if err != nil {
			lastErr, have5xx, status = err, false, 0
			continue // 网络错误：可重试
		}
		b, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			lastErr, have5xx, status = rerr, false, 0
			continue // 读错误：可重试
		}
		if resp.StatusCode >= 500 {
			have5xx, status, last5xx, lastErr = true, resp.StatusCode, b, nil
			continue // 5xx：可重试
		}
		return resp.StatusCode, b, nil // 2xx/4xx：不重试
	}
	if have5xx {
		return status, last5xx, nil
	}
	return 0, nil, fmt.Errorf("http helper: %w", lastErr)
}
