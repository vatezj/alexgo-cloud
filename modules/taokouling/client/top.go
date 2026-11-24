package client

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrTpwdCreate 表示 TOP 生成淘口令业务失败：响应不可解析、含 error_response、或缺 data.model。
// 业务失败不触发 HTTPHelper 重试（单次尝试即返回）。
var ErrTpwdCreate = errors.New("top: 淘口令生成失败")

// ErrItemInfo 表示 TOP 商品信息业务失败：响应不可解析、含 error_response、
// 或 results.n_tbk_item 缺失/空数组/缺 title。同样不重试。
var ErrItemInfo = errors.New("top: 商品信息缺失")

// Item 是 TOP 商品信息（标题/主图/折扣价/划线价），Task 12 文案与返利计算消费。
type Item struct {
	Title        string
	PictURL      string
	ZkPrice      float64
	ReservePrice float64
}

// TopClient 是淘宝官方 TOP（taobao.*）客户端：md5 签名自实现，不依赖官方 SDK。
type TopClient struct {
	helper  *HTTPHelper
	baseURL string
	appkey  string
	secret  string
	userID  string
}

// NewTopClient 构造客户端。maxRetries 为「重试次数」（总尝试 = 1 + maxRetries）。
func NewTopClient(baseURL, appkey, secret, userID string, timeout time.Duration, maxRetries int) *TopClient {
	return &TopClient{
		helper:  NewHTTPHelper(timeout, maxRetries),
		baseURL: strings.TrimRight(baseURL, "/"),
		appkey:  appkey,
		secret:  secret,
		userID:  userID,
	}
}

// topSign 计算 TOP md5 签名（小写、同包测试直接访问）：
// 剔除 sign 自身 → 剩余 key 按字节升序 → 依次拼 key+value（value 用原始串，不 urlencode）
// → sign = md5(secret + 拼接串 + secret) 的小写 hex。
func topSign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys) // 字节序升序

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	sum := md5.Sum([]byte(secret + b.String() + secret))
	return hex.EncodeToString(sum[:])
}

// postForm 补 sign 后以 application/x-www-form-urlencoded POST {base}（公共参数与业务参数一并进签名集合）。
// 返回 HTTP 状态码与响应体；传输层失败（网络错/5xx 重试耗尽）返回 error。
func (c *TopClient) postForm(ctx context.Context, params map[string]string) (int, []byte, error) {
	params["sign"] = topSign(params, c.secret)
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	return c.helper.Post(ctx, c.baseURL, "application/x-www-form-urlencoded", []byte(form.Encode()))
}

// topErrField 为 TOP 统一业务错误节点（error_response 可能与成功字段并存于顶层）。
type topErrField struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// topBusinessErr 将 error_response 归一为哨兵错误（保留 code/msg 于文案）。
func topBusinessErr(sentinel error, e *topErrField) error {
	return fmt.Errorf("%w: %s %s", sentinel, e.Code, e.Msg)
}

// TpwdCreate 生成自己的新淘口令：POST {base}，sys=taobao.tbk.tpwd.create，
// 参数 user_id（构造传入）/text/url/logo（空则整键省略，不进签名集合）/ext="{}"/format=json
// 及公共参数 appkey/sign_method=md5。成功取 data.model（"￥xxx￥"）。
// 业务失败（缺 data.model / error_response / 响应不可解析）→ ErrTpwdCreate，不重试；
// 传输层失败（网络错 / 5xx 重试耗尽 / 4xx）→ 其他 error。
func (c *TopClient) TpwdCreate(ctx context.Context, text, url, logo string) (string, error) {
	params := map[string]string{
		"sys":         "taobao.tbk.tpwd.create",
		"appkey":      c.appkey,
		"sign_method": "md5",
		"user_id":     c.userID,
		"text":        text,
		"url":         url,
		"ext":         "{}",
		"format":      "json",
	}
	if logo != "" {
		params["logo"] = logo
	}

	status, body, err := c.postForm(ctx, params)
	if err != nil {
		return "", fmt.Errorf("top: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("top: HTTP 状态码 %d", status)
	}

	var resp struct {
		ErrorResponse *topErrField `json:"error_response"`
		Data          *struct {
			Model string `json:"model"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", ErrTpwdCreate
	}
	if resp.ErrorResponse != nil {
		return "", topBusinessErr(ErrTpwdCreate, resp.ErrorResponse)
	}
	if resp.Data == nil || resp.Data.Model == "" {
		return "", ErrTpwdCreate
	}
	return resp.Data.Model, nil
}

// topItemRaw 为 n_tbk_item 元素：价格字段兼容 JSON 字符串与数字（RawMessage 后判）。
type topItemRaw struct {
	Title        string          `json:"title"`
	PictURL      string          `json:"pict_url"`
	ZkPrice      json.RawMessage `json:"zk_final_price"`
	ReservePrice json.RawMessage `json:"reserve_price"`
}

// topItemResults 为 results 节点：n_tbk_item 是数组。
type topItemResults struct {
	NTbkItem []topItemRaw `json:"n_tbk_item"`
}

// parseTopPrice 将价格字面量转 float64：带引号字符串去引号、数字直取；
// 空串/null → 0；不可解析 → error。
func parseTopPrice(raw json.RawMessage) (float64, error) {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return 0, nil
	}
	s = strings.Trim(s, `"`)
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("top: 价格 %q 不可解析: %w", s, err)
	}
	return v, nil
}

// ItemInfo 查商品信息：POST {base}，sys=taobao.tbk.item.info.get，
// num_iids=itemID、platform=1、ip=clientIP 及公共参数 + sign。
// 响应 results.n_tbk_item 为数组，取 [0] 的 title/pict_url/zk_final_price/reserve_price
// （兼容外层 tbk_item_info_get_response 包装与裸 results 两种形状）。
// 业务失败（空数组 / 缺 title / 价格不可解析 / error_response / 不可解析）→ ErrItemInfo，不重试；
// 传输层失败（网络错 / 5xx 重试耗尽 / 4xx）→ 其他 error。
func (c *TopClient) ItemInfo(ctx context.Context, itemID, clientIP string) (Item, error) {
	params := map[string]string{
		"sys":         "taobao.tbk.item.info.get",
		"appkey":      c.appkey,
		"sign_method": "md5",
		"num_iids":    itemID,
		"platform":    "1",
		"ip":          clientIP,
		"format":      "json",
	}

	status, body, err := c.postForm(ctx, params)
	if err != nil {
		return Item{}, fmt.Errorf("top: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return Item{}, fmt.Errorf("top: HTTP 状态码 %d", status)
	}

	var resp struct {
		ErrorResponse          *topErrField `json:"error_response"`
		TbkItemInfoGetResponse *struct {
			Results *topItemResults `json:"results"`
		} `json:"tbk_item_info_get_response"`
		Results *topItemResults `json:"results"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return Item{}, ErrItemInfo
	}
	if resp.ErrorResponse != nil {
		return Item{}, topBusinessErr(ErrItemInfo, resp.ErrorResponse)
	}

	results := resp.Results
	if results == nil && resp.TbkItemInfoGetResponse != nil {
		results = resp.TbkItemInfoGetResponse.Results
	}
	if results == nil || len(results.NTbkItem) == 0 {
		return Item{}, ErrItemInfo
	}
	raw := results.NTbkItem[0]
	if raw.Title == "" {
		return Item{}, ErrItemInfo
	}
	zk, err := parseTopPrice(raw.ZkPrice)
	if err != nil {
		return Item{}, fmt.Errorf("%w: %v", ErrItemInfo, err)
	}
	reserve, err := parseTopPrice(raw.ReservePrice)
	if err != nil {
		return Item{}, fmt.Errorf("%w: %v", ErrItemInfo, err)
	}
	return Item{
		Title:        raw.Title,
		PictURL:      raw.PictURL,
		ZkPrice:      zk,
		ReservePrice: reserve,
	}, nil
}
