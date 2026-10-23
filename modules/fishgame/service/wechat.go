package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WechatClient 微信小游戏 code2session（与 oauth2 social 渠道解耦，spec §6）。
type WechatClient struct {
	baseURL string
	client  *http.Client
	appid   string
	secret  string
}

// NewWechatClient 默认 5s 超时（brief 硬性要求，防微信侧慢响应拖死登录）。
func NewWechatClient(appid, secret string) *WechatClient {
	return &WechatClient{
		baseURL: "https://api.weixin.qq.com",
		client:  &http.Client{Timeout: 5 * time.Second},
		appid:   appid,
		secret:  secret,
	}
}

// Exchange 用 js_code 换 openid。错误文本绝不含 secret（照 social/wechat_mp.go 教训）。
func (c *WechatClient) Exchange(ctx context.Context, code string) (string, error) {
	q := url.Values{
		"appid":      {c.appid},
		"secret":     {c.secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		// client.Do 失败时错误是 *url.Error，其 Error() 含完整 URL（含 secret 查询参数）；
		// 只保留内层 Err，防止 secret 随 error 文本泄入日志。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return "", fmt.Errorf("wechat code2session: request failed: %w", urlErr.Err)
		}
		return "", fmt.Errorf("wechat code2session: request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("wechat code2session: bad json: %w", err)
	}
	if out.ErrCode != 0 || out.OpenID == "" {
		return "", fmt.Errorf("wechat code2session: errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	return out.OpenID, nil
}
