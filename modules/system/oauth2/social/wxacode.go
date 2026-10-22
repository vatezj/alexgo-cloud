package social

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AppGetter 取渠道配置窄接口（对外暴露）：SocialRepository.GetApp 即实现，
// scorecard 小程序码控制器经其 module 注入消费（不直依赖 12 方法大接口）。
type AppGetter interface {
	GetApp(ctx context.Context, tenantID uint64, platform string) (*SocialApp, error)
}

// Wxacoder 小程序码生成窄接口（对外暴露）：*WechatMP 实现，scorecard module 注入。
type Wxacoder interface {
	Wxacode(ctx context.Context, app *SocialApp, scene, page, envVersion string) ([]byte, error)
}

// stableTokenSkew 进程内缓存提前失效量：到期前 300s 即视为过期，
// 防边界上拿到将死 token（真实可用窗口 = expires_in - 300s）。
const stableTokenSkew = 300 * time.Second

// mpToken stable_token 进程内缓存条目（按 appid；单实例足够，不引入 redis）。
type mpToken struct {
	token string
	expAt time.Time
}

// StableToken 换取 stable access_token：POST /cgi-bin/stable_token
// （body {grant_type:"client_credential",appid,secret}）。
// 成功后按 appid 缓存至 expires_in-300s，缓存期内不再打接口；
// 并发同 appid 串行取（取数持锁，量小可接受）。
func (m *WechatMP) StableToken(ctx context.Context, app *SocialApp) (string, error) {
	m.tokMu.Lock()
	defer m.tokMu.Unlock()
	if m.tokCache == nil {
		m.tokCache = make(map[string]mpToken)
	}
	if e, ok := m.tokCache[app.AppID]; ok && time.Now().Before(e.expAt) {
		return e.token, nil
	}
	payload, err := json.Marshal(map[string]string{
		"grant_type": "client_credential",
		"appid":      app.AppID,
		"secret":     app.Secret,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.baseURL+"/cgi-bin/stable_token", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		// 沿用 Exchange 的剥离先例（secret 在 body、不在 URL，仍只保留内层 Err 防串含 URL）。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return "", fmt.Errorf("wechat mp stable_token: request failed (network): %w", urlErr.Err)
		}
		return "", fmt.Errorf("wechat mp stable_token: request failed (network): %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("wechat mp stable_token: bad json: %w", err)
	}
	// 平台原始错误（errcode/errmsg 无 secret）只上抛给调用方记日志，不含敏感参数。
	if out.ErrCode != 0 || out.AccessToken == "" {
		return "", fmt.Errorf("wechat mp stable_token: errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	// expires_in≤300s 时 expAt 已过期 → 永不命中缓存（每次现取），不会缓存将死 token。
	m.tokCache[app.AppID] = mpToken{
		token: out.AccessToken,
		expAt: time.Now().Add(time.Duration(out.ExpiresIn)*time.Second - stableTokenSkew),
	}
	return out.AccessToken, nil
}

// Wxacode 生成小程序码：POST /wxa/getwxacodeunlimit?access_token=...
// body {scene,page,env_version,check_path:false}。
// 成功响应体即图片字节（image/png）原样透传；失败响应体是 JSON（errcode≠0）→ 上抛。
// envVersion 空=正式版（本方法不下发 env_version 键，微信默认 release），"trial"=体验版。
func (m *WechatMP) Wxacode(ctx context.Context, app *SocialApp, scene, page, envVersion string) ([]byte, error) {
	token, err := m.StableToken(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("wechat mp getwxacodeunlimit: stable token: %w", err)
	}
	payload, err := json.Marshal(struct {
		Scene      string `json:"scene"`
		Page       string `json:"page,omitempty"`
		EnvVersion string `json:"env_version,omitempty"` // 空=正式版（不下发，微信默认 release）
		CheckPath  bool   `json:"check_path"`
	}{Scene: scene, Page: page, EnvVersion: envVersion})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		m.baseURL+"/wxa/getwxacodeunlimit?access_token="+url.QueryEscape(token), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		// client.Do 失败时 *url.Error.Error() 含完整 URL（此处内嵌 access_token），
		// 只保留内层 Err，防止 token 随 error 文本泄入日志（同 Exchange 剥离先例）。
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, fmt.Errorf("wechat mp getwxacodeunlimit: request failed (network): %w", urlErr.Err)
		}
		return nil, fmt.Errorf("wechat mp getwxacodeunlimit: request failed (network): %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 图片量级小，8MB 保险丝
	if err != nil {
		return nil, err
	}
	// 成功是二进制图片、失败是 JSON：能解析出非 0 errcode 才按错误上抛，
	// 否则原样透传字节（图片体解析 JSON 必失败）。
	var ec struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if json.Unmarshal(body, &ec) == nil && ec.ErrCode != 0 {
		return nil, fmt.Errorf("wechat mp getwxacodeunlimit: errcode=%d errmsg=%s", ec.ErrCode, ec.ErrMsg)
	}
	return body, nil
}
