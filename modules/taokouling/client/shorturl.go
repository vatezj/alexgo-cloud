package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// ShortURLClient 是 ft12 短链客户端：单次尝试（无重试配置），超时由构造传入（配置 3s）。
type ShortURLClient struct {
	helper  *HTTPHelper
	baseURL string
	apikey  string
}

// NewShortURLClient 构造客户端（maxRetries 固定 0——短链失败由调用方回退长链，不重试）。
func NewShortURLClient(baseURL, apikey string, timeout time.Duration) *ShortURLClient {
	return &ShortURLClient{
		helper:  NewHTTPHelper(timeout, 0),
		baseURL: baseURL,
		apikey:  apikey,
	}
}

// Shorten 缩短长链：GET {base}?format=json&url={urlencoded longURL}&apikey={key}
// → {"url":"..."}；缺 url 字段（或响应不可解析/HTTP 非 2xx）→ error。
func (c *ShortURLClient) Shorten(ctx context.Context, longURL string) (string, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("shorturl: 非法 baseURL: %w", err)
	}
	q := u.Query()
	q.Set("format", "json")
	q.Set("url", longURL)
	q.Set("apikey", c.apikey)
	u.RawQuery = q.Encode()

	status, body, err := c.helper.Get(ctx, u.String())
	if err != nil {
		return "", fmt.Errorf("shorturl: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("shorturl: HTTP 状态码 %d", status)
	}

	var resp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("shorturl: 响应解析失败: %w", err)
	}
	if resp.URL == "" {
		return "", errors.New("shorturl: 响应缺少 url 字段")
	}
	return resp.URL, nil
}
