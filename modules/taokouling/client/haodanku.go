package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrNoRates 表示 haodanku 换链业务失败：code!=1，或成功链路缺 coupon_click_url。
// 业务失败不触发 HTTPHelper 重试（单次尝试即返回）。
var ErrNoRates = errors.New("haodanku: 换链失败")

// HaodankuClient 是好单库（haodanku）换链与商品详情客户端。
type HaodankuClient struct {
	helper  *HTTPHelper
	baseURL string
}

// NewHaodankuClient 构造客户端。maxRetries 为「重试次数」（总尝试 = 1 + maxRetries）。
func NewHaodankuClient(baseURL string, timeout time.Duration, maxRetries int) *HaodankuClient {
	return &HaodankuClient{
		helper:  NewHTTPHelper(timeout, maxRetries),
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// haodankuRatesResponse 为 POST /ratesurl 的响应结构。
// code 兼容数字 1 与字符串 "1"（RawMessage 后判）；couponmoney 用 json.Number 容忍小数面额。
type haodankuRatesResponse struct {
	Code json.RawMessage `json:"code"`
	Data *struct {
		CouponClickURL string      `json:"coupon_click_url"`
		CouponMoney    json.Number `json:"couponmoney"`
	} `json:"data"`
}

// ratesCodeIsOne 判定 code==1（数字 1 或字符串 "1"）。
func ratesCodeIsOne(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "1" || s == `"1"`
}

// RatesURL 换取带券推广链接：POST {base}/ratesurl，
// application/x-www-form-urlencoded，字段 pid/apikey/tb_name/itemid。
// code==1 且有 coupon_click_url → 成功（coupon 取 data.couponmoney，可为 0）。
// 业务失败（code!=1 / 缺链接 / 响应不可解析）→ ErrNoRates，不重试。
// 传输层失败（网络错 / 5xx 重试耗尽 / 4xx）→ 其他 error，重试按 HTTPHelper 策略。
func (c *HaodankuClient) RatesURL(ctx context.Context, pid, apikey, tbName, itemID string) (string, int, error) {
	form := url.Values{}
	form.Set("pid", pid)
	form.Set("apikey", apikey)
	form.Set("tb_name", tbName)
	form.Set("itemid", itemID)

	status, body, err := c.helper.Post(ctx, c.baseURL+"/ratesurl",
		"application/x-www-form-urlencoded", []byte(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("haodanku: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", 0, fmt.Errorf("haodanku: HTTP 状态码 %d", status)
	}

	var resp haodankuRatesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", 0, ErrNoRates
	}
	if !ratesCodeIsOne(resp.Code) || resp.Data == nil || resp.Data.CouponClickURL == "" {
		return "", 0, ErrNoRates
	}
	return resp.Data.CouponClickURL, parseCouponMoney(resp.Data.CouponMoney), nil
}

// parseCouponMoney 将 couponmoney 字面量转 int；空或不可解析按 0（链接是主结果，不因此失败）。
func parseCouponMoney(n json.Number) int {
	s := n.String()
	if s == "" {
		return 0
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int(f)
	}
	return 0
}

// ItemDesc 取商品描述（必买理由）：GET {base}/item_detail/apikey/{apikey}/itemid/{itemID}
// （apikey/itemID 两段均经 url.PathEscape）→ data.itemdesc。
// apikey 属渠道三元组，由服务层（Task 12）按已解析渠道在调用时传入——
// 合作渠道 apikey 各不相同，客户端不得硬编码。
// 调用方容忍失败（err → 文案理由行省略）；传输层/5xx 按 HTTPHelper 策略重试。
func (c *HaodankuClient) ItemDesc(ctx context.Context, itemID, apikey string) (string, error) {
	u := c.baseURL + "/item_detail/apikey/" +
		url.PathEscape(apikey) + "/itemid/" + url.PathEscape(itemID)

	status, body, err := c.helper.Get(ctx, u)
	if err != nil {
		return "", fmt.Errorf("haodanku: 请求失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("haodanku: HTTP 状态码 %d", status)
	}

	var resp struct {
		Data *struct {
			ItemDesc string `json:"itemdesc"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("haodanku: 详情解析失败: %w", err)
	}
	if resp.Data == nil || resp.Data.ItemDesc == "" {
		return "", errors.New("haodanku: 商品详情缺失 itemdesc")
	}
	return resp.Data.ItemDesc, nil
}
