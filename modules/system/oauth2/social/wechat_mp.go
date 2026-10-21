package social

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WechatMP 微信小程序 code2session。
type WechatMP struct {
	baseURL string
	client  *http.Client
}

func NewWechatMP() *WechatMP {
	return &WechatMP{baseURL: "https://api.weixin.qq.com", client: &http.Client{Timeout: 10 * time.Second}}
}

func (m *WechatMP) Platform() string { return PlatformWechatMP }

func (m *WechatMP) Exchange(ctx context.Context, app *SocialApp, code string) (*Identity, error) {
	q := url.Values{
		"appid":      {app.AppID},
		"secret":     {app.Secret},
		"js_code":    {code},
		"grant_type": {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/sns/jscode2session?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat mp code2session: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var out struct {
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("wechat mp code2session: bad json: %w", err)
	}
	// 平台原始错误只返回给调用方映射 30302，errcode 不上抛给客户端（spec §3）
	if out.ErrCode != 0 || out.OpenID == "" {
		return nil, fmt.Errorf("wechat mp code2session: errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	return &Identity{OpenID: out.OpenID, UnionID: out.UnionID}, nil
}
