package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ErrParseFailed 表示 zhetaoke 业务失败：顶层含 error_response，或成功链路任一层缺失（含缺 item_id）。
var ErrParseFailed = errors.New("zhetaoke: 口令解析失败")

const zhetaokeParsePath = "/api/open_gaoyongzhuanlian_tkl.ashx"

// ZhetaokeClient 是折搞定（zhetaoke）口令解析客户端。
type ZhetaokeClient struct {
	helper   *HTTPHelper
	baseURL  string
	appkey   string
	sid      string
	parsePID string
}

// NewZhetaokeClient 构造客户端。maxRetries 为「重试次数」（总尝试 = 1 + maxRetries）。
func NewZhetaokeClient(baseURL, appkey, sid, parsePID string, timeout time.Duration, maxRetries int) *ZhetaokeClient {
	return &ZhetaokeClient{
		helper:   NewHTTPHelper(timeout, maxRetries),
		baseURL:  strings.TrimRight(baseURL, "/"),
		appkey:   appkey,
		sid:      sid,
		parsePID: parsePID,
	}
}

// zhetaokeParseResponse 为 signurl=0 轻量链路的响应结构。
type zhetaokeParseResponse struct {
	ErrorResponse          json.RawMessage `json:"error_response"`
	TbkPrivilegeGetResponse *struct {
		Result *struct {
			Data *struct {
				ItemID json.Number `json:"item_id"`
			} `json:"data"`
		} `json:"result"`
	} `json:"tbk_privilege_get_response"`
}

// ParseTkl 解析口令，返回 item_id（JSON number → string）。
// 业务失败返回 ErrParseFailed；传输层失败（网络错/5xx 重试耗尽/4xx）返回其他 error。
func (c *ZhetaokeClient) ParseTkl(ctx context.Context, tkl string) (string, error) {
	u, err := url.Parse(c.baseURL + zhetaokeParsePath)
	if err != nil {
		return "", fmt.Errorf("zhetaoke: 非法 baseURL: %w", err)
	}
	q := u.Query()
	q.Set("appkey", c.appkey)
	q.Set("sid", c.sid)
	q.Set("pid", c.parsePID)
	q.Set("tkl", tkl)
	q.Set("signurl", "0")
	u.RawQuery = q.Encode()

	status, body, err := c.helper.Get(ctx, u.String())
	if err != nil {
		return "", fmt.Errorf("zhetaoke: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("zhetaoke: HTTP 状态码 %d", status)
	}

	var resp zhetaokeParseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ErrParseFailed
	}
	if len(resp.ErrorResponse) > 0 && !bytes.Equal(resp.ErrorResponse, []byte("null")) {
		return "", ErrParseFailed
	}
	if resp.TbkPrivilegeGetResponse == nil ||
		resp.TbkPrivilegeGetResponse.Result == nil ||
		resp.TbkPrivilegeGetResponse.Result.Data == nil {
		return "", ErrParseFailed
	}
	itemID := resp.TbkPrivilegeGetResponse.Result.Data.ItemID.String()
	if itemID == "" {
		return "", ErrParseFailed
	}
	return itemID, nil
}
