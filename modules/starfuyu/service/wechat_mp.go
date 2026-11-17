package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"sync"
	"time"

	errs "alexGo-cloud/pkg/errors"
)

// wechatBaseURL 微信公众号 API 根（测试用 baseURL 字段覆盖为 httptest）。
const wechatBaseURL = "https://api.weixin.qq.com"

// tokenEarlyRefresh access_token 提前刷新窗口（照 goLucky service/wechat_mp.go：到期前 5 分钟重取）。
const tokenEarlyRefresh = 5 * time.Minute

// MPClient 微信公众号客户端（草稿箱路径）。
// 合同：所有方法失败一律包 30507（errs.CodeStarfuyuMpUploadFailed）。
type MPClient interface {
	// UploadImageMaterial 上传永久图片素材（controller 裁定：收 bytes 非文件路径，
	// ArticleService 经 storage.Driver.Open(key) 读字节后传入）。
	UploadImageMaterial(ctx context.Context, data []byte) (mediaID string, err error)
	// UploadContentImage 正文图片上传（/cgi-bin/media/uploadimg），返回可嵌正文的微信 CDN URL。
	// 与 UploadImageMaterial（永久素材，回 media_id 供封面）分工：正文图必须走本方法。
	UploadContentImage(ctx context.Context, data []byte) (url string, err error)
	// AddDraft 新建草稿（articles 至少 1 条），返回草稿 media_id。
	AddDraft(ctx context.Context, articles []MPArticle) (mediaID string, err error)
	// ListDrafts 分页列草稿（draft/batchget，no_content=1 只回元数据），返回 total 与条目。
	ListDrafts(ctx context.Context, offset, count int) (total int, items []DraftMeta, err error)
	// DeleteDraft 删除单条草稿（draft/delete，不可逆）。失败一律 30507。
	DeleteDraft(ctx context.Context, mediaID string) error
}

// DraftMeta 草稿条目元数据（draft/batchget item；只取清理工具要用的字段）。
type DraftMeta struct {
	MediaID string `json:"media_id"`
	Content struct {
		Title    string `json:"title"`
		NewsItem []struct {
			Title string `json:"title"`
		} `json:"news_item"`
	} `json:"content"`
}

// Title 首篇文章标题：content.title（单层）或 content.news_item[0].title（图文结构）。
func (d DraftMeta) Title() string {
	if d.Content.Title != "" {
		return d.Content.Title
	}
	if len(d.Content.NewsItem) > 0 {
		return d.Content.NewsItem[0].Title
	}
	return ""
}

// MPArticle 草稿箱文章条目（字段对齐微信 draft/add articles[]）。
type MPArticle struct {
	Title        string `json:"title"`
	Content      string `json:"content"`
	Digest       string `json:"digest,omitempty"`
	Author       string `json:"author,omitempty"`
	ShowCoverPic int    `json:"show_cover_pic"`
	ThumbMediaID string `json:"thumb_media_id,omitempty"`
}

type wechatMPClient struct {
	appID     string
	appSecret string
	baseURL   string
	client    *http.Client

	mu            sync.RWMutex
	accessToken   string
	tokenExpireAt time.Time
}

// NewWeChatMPClient 构造公众号客户端。access_token 进程内缓存照 goLucky（提前 5 分钟刷新）。
// appID/appSecret 空配时所有调用直接 30507、不打网络。
func NewWeChatMPClient(appID, appSecret string) MPClient {
	return &wechatMPClient{
		appID:     appID,
		appSecret: appSecret,
		baseURL:   wechatBaseURL,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func mpFail(format string, args ...any) error {
	return errs.New(errs.CodeStarfuyuMpUploadFailed, fmt.Errorf(format, args...))
}

// getAccessToken 取缓存 token；缺失/临近过期（5 分钟内）则回源刷新。
func (c *wechatMPClient) getAccessToken(ctx context.Context) (string, error) {
	if c.appID == "" || c.appSecret == "" {
		return "", mpFail("wechat_mp appid/secret 未配置")
	}

	c.mu.RLock()
	if c.accessToken != "" && time.Now().Before(c.tokenExpireAt.Add(-tokenEarlyRefresh)) {
		tok := c.accessToken
		c.mu.RUnlock()
		return tok, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// 双重检查（照 goLucky）。
	if c.accessToken != "" && time.Now().Before(c.tokenExpireAt.Add(-tokenEarlyRefresh)) {
		return c.accessToken, nil
	}

	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", c.appID)
	q.Set("secret", c.appSecret)
	full := fmt.Sprintf("%s/cgi-bin/token?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return "", mpFail("构造 token 请求: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", mpFail("请求微信 token 失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", mpFail("读取 token 响应失败: %w", err)
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", mpFail("解析 token 响应失败: %w", err)
	}
	if result.ErrCode != 0 || result.AccessToken == "" {
		return "", mpFail("微信 token errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	c.accessToken = result.AccessToken
	c.tokenExpireAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

// doJSON POST JSON 到 cgi-bin 路径，统一解包 errcode/media_id。
func (c *wechatMPClient) doJSON(ctx context.Context, path string, payload any) ([]byte, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, mpFail("序列化请求失败: %w", err)
	}
	full := fmt.Sprintf("%s%s?access_token=%s", c.baseURL, path, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, bytes.NewReader(data))
	if err != nil {
		return nil, mpFail("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, mpFail("请求微信失败 %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, mpFail("读取响应失败 %s: %w", path, err)
	}
	return body, nil
}

// checkErrOnly 只解 {errcode,errmsg}：errcode≠0 → 30507；0 → nil。
// 供不回 media_id 的端点用（draft/delete、draft/batchget——checkErrCode 会误报缺 media_id）。
func checkErrOnly(path string, body []byte) error {
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return mpFail("解析 %s 响应失败: %w", path, err)
	}
	if result.ErrCode != 0 {
		return mpFail("微信API错误 %s: errcode=%d errmsg=%s", path, result.ErrCode, result.ErrMsg)
	}
	return nil
}

// checkErrCode 解包 {errcode,errmsg,media_id}；errcode≠0 → 30507。
func checkErrCode(path string, body []byte) (mediaID string, err error) {
	var result struct {
		MediaID string `json:"media_id"`
		URL     string `json:"url"`
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", mpFail("解析 %s 响应失败: %w", path, err)
	}
	if result.ErrCode != 0 {
		return "", mpFail("微信API错误 %s: errcode=%d errmsg=%s", path, result.ErrCode, result.ErrMsg)
	}
	if result.MediaID == "" {
		return "", mpFail("%s 响应缺 media_id", path)
	}
	return result.MediaID, nil
}

// UploadImageMaterial 上传永久图片素材（multipart media 字段，收 bytes）。
func (c *wechatMPClient) UploadImageMaterial(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 {
		return "", mpFail("图片字节为空")
	}
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("media", "thumb.jpg")
	if err != nil {
		return "", mpFail("创建素材表单字段失败: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", mpFail("写入素材字节失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", mpFail("关闭素材 multipart 失败: %w", err)
	}

	full := fmt.Sprintf("%s/cgi-bin/material/add_material?access_token=%s&type=image",
		c.baseURL, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, &buf)
	if err != nil {
		return "", mpFail("构造素材请求失败: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", mpFail("上传图片素材失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", mpFail("读取素材响应失败: %w", err)
	}
	return checkErrCode("/cgi-bin/material/add_material", body)
}

// UploadContentImage 正文图片上传（media/uploadimg）：与 add_material 同 multipart 形态，
// 响应取 url（微信 CDN，可直接嵌草稿 content）。合同：失败一律 30507。
func (c *wechatMPClient) UploadContentImage(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 {
		return "", mpFail("图片字节为空")
	}
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("media", "image.jpg")
	if err != nil {
		return "", mpFail("创建图片表单字段失败: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", mpFail("写入图片字节失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", mpFail("关闭图片 multipart 失败: %w", err)
	}

	full := fmt.Sprintf("%s/cgi-bin/media/uploadimg?access_token=%s",
		c.baseURL, url.QueryEscape(token))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, &buf)
	if err != nil {
		return "", mpFail("构造正文图片请求失败: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return "", mpFail("上传正文图片失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", mpFail("读取正文图片响应失败: %w", err)
	}
	var out struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", mpFail("解析正文图片响应失败: %w, 预览: %s", err, truncate(string(body), 200))
	}
	if out.URL == "" {
		// errcode≠0 或缺 url 都走统一错误通道（30507）。
		if _, err := checkErrCode("/cgi-bin/media/uploadimg", body); err != nil {
			return "", err
		}
		return "", mpFail("正文图片响应缺 url: %s", truncate(string(body), 200))
	}
	return out.URL, nil
}

// ListDrafts 分页列草稿：POST draft/batchget {offset,count,no_content}。
// 合同：失败一律 30507。
func (c *wechatMPClient) ListDrafts(ctx context.Context, offset, count int) (int, []DraftMeta, error) {
	body, err := c.doJSON(ctx, "/cgi-bin/draft/batchget", map[string]any{
		"offset": offset, "count": count, "no_content": 1,
	})
	if err != nil {
		return 0, nil, err
	}
	if err := checkErrOnly("/cgi-bin/draft/batchget", body); err != nil {
		return 0, nil, err
	}
	var out struct {
		TotalCount int         `json:"total_count"`
		Item       []DraftMeta `json:"item"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, nil, mpFail("解析草稿列表失败: %w, 预览: %s", err, truncate(string(body), 200))
	}
	return out.TotalCount, out.Item, nil
}

// DeleteDraft 删除草稿（draft/delete，不可逆）。合同：失败一律 30507。
func (c *wechatMPClient) DeleteDraft(ctx context.Context, mediaID string) error {
	if mediaID == "" {
		return mpFail("media_id 为空")
	}
	body, err := c.doJSON(ctx, "/cgi-bin/draft/delete", map[string]any{"media_id": mediaID})
	if err != nil {
		return err
	}
	return checkErrOnly("/cgi-bin/draft/delete", body)
}

// AddDraft 新建草稿（个人订阅号草稿箱接口，照 goLucky）。
func (c *wechatMPClient) AddDraft(ctx context.Context, articles []MPArticle) (string, error) {
	if len(articles) == 0 {
		return "", mpFail("articles 为空")
	}
	body, err := c.doJSON(ctx, "/cgi-bin/draft/add", struct {
		Articles []MPArticle `json:"articles"`
	}{Articles: articles})
	if err != nil {
		return "", err
	}
	return checkErrCode("/cgi-bin/draft/add", body)
}
