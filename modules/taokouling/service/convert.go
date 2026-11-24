package service

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"alexGo-cloud/modules/taokouling/client"
	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/tenant"

	"go.uber.org/fx"
)

// TklParser 淘口令解析依赖（client.ZhetaokeClient 结构化满足）。
type TklParser interface {
	ParseTkl(ctx context.Context, tkl string) (string, error)
}

// RatesClient 换链/商品理由依赖（client.HaodankuClient 结构化满足）。
// R7：ItemDesc 的 apikey 按解析到的渠道传入，禁硬编码。
type RatesClient interface {
	RatesURL(ctx context.Context, pid, apikey, tbName, itemID string) (string, int, error)
	ItemDesc(ctx context.Context, itemID, apikey string) (string, error)
}

// TopClient 淘宝 TOP 依赖（client.TopClient 结构化满足；Item 用 client 包类型）。
type TopClient interface {
	TpwdCreate(ctx context.Context, text, url, logo string) (string, error)
	ItemInfo(ctx context.Context, itemID, clientIP string) (client.Item, error)
}

// Shortener 短链依赖（client.ShortURLClient 结构化满足）。
type Shortener interface {
	Shorten(ctx context.Context, longURL string) (string, error)
}

// LogStore 转换日志追加依赖（repository.ConversionLogRepository.Append 逐字满足）。
type LogStore interface {
	Append(ctx context.Context, rec *model.ConversionLog) error
}

// ConvertResult 完整链路返回（spec §6）。
type ConvertResult struct {
	Text        string `json:"text"`
	ItemID      string `json:"item_id"`
	Degraded    bool   `json:"degraded"`
	Title       string `json:"title,omitempty"`
	CouponMoney int    `json:"coupon_money,omitempty"`
	NewTkl      string `json:"new_tkl,omitempty"`
}

// ConvertParams 依赖聚合（fx.In）。
type ConvertParams struct {
	fx.In
	Parser   TklParser
	Rates    RatesClient
	Top      TopClient
	Short    Shortener
	Channels ChannelStore
	Logs     LogStore
	Cfg      *config.Config
	Cache    *TTLCache
}

// ConversionLog.Status 取值（1 成功 / 2 降级 / 3 无口令 / 4 硬错误）。
const (
	logStatusOK       int8 = 1
	logStatusDegraded int8 = 2
	logStatusNoTkl    int8 = 3
	logStatusHardErr  int8 = 4
)

// 端点名（model.ConversionLog.Endpoint）。
const (
	endpointParse   = "parse"
	endpointConvert = "convert"
)

// ratesEntry rates 缓存值（key=rates:{itemID}:{pid}）。
type ratesEntry struct {
	ClickURL string
	Coupon   int
}

// defaultCacheTTL CacheTTLSeconds<=0 时的兜底 TTL（spec：1800s）。
const defaultCacheTTL = 1800 * time.Second

// ConversionService Parse/Convert 编排（spec §6）。
type ConversionService struct {
	parser   TklParser
	rates    RatesClient
	top      TopClient
	short    Shortener
	channels ChannelStore
	logs     LogStore
	cfg      *config.Config
	cache    *TTLCache
}

// NewConversionService 构造编排服务；Cfg 为 nil 时兜底空配置（配置检查按缺失处理）。
func NewConversionService(p ConvertParams) *ConversionService {
	cfg := p.Cfg
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &ConversionService{
		parser:   p.Parser,
		rates:    p.Rates,
		top:      p.Top,
		short:    p.Short,
		channels: p.Channels,
		logs:     p.Logs,
		cfg:      cfg,
		cache:    p.Cache,
	}
}

// Parse 轻量链路：重包 ctx(tid=0) → Extract（无 → 30701，log status=3）
// → zhetaoke.appkey 空 → 30702（log 4）→ ParseTkl（ErrParseFailed/传输终败 → 30705，log 4）
// → 成功 log status=1（endpoint=parse）→ 返回 itemID。clientIP 记日志。
// 每条路径恰落一条 ConversionLog；不查渠道、不走降级。
func (s *ConversionService) Parse(ctx context.Context, password, partner, clientIP string) (string, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，恒钉 tid=0
	start := time.Now()

	tkl, ok := Extract(password)
	if !ok {
		s.appendLog(ctx, endpointParse, partner, password, "", "", logStatusNoTkl, nil, clientIP, start)
		return "", errs.New(errs.CodeTaoKoulingPasswordNotFound, nil)
	}
	if s.cfg.Taokouling.Zhetaoke.Appkey == "" {
		s.appendLog(ctx, endpointParse, partner, password, tkl, "", logStatusHardErr, nil, clientIP, start)
		return "", errs.New(errs.CodeTaoKoulingConfigMissing, nil)
	}

	itemID, pErr := s.parser.ParseTkl(ctx, tkl)
	if pErr != nil {
		// 业务失败（ErrParseFailed）与传输终败统一 30705 硬错（客户端已内置重试）。
		s.appendLog(ctx, endpointParse, partner, password, tkl, "", logStatusHardErr, pErr, clientIP, start)
		return "", errs.New(errs.CodeTaoKoulingParseFailed, pErr)
	}

	s.appendLog(ctx, endpointParse, partner, password, tkl, itemID, logStatusOK, nil, clientIP, start)
	return itemID, nil
}

// Convert 完整链路（spec §6 编排）：重包 ctx → Extract（无 → 30701，log 3）
// → 配置检查（zhetaoke/top appkey·secret 空 → 30702，log 4）
// → Resolve（错 → 原样上抛，log 4）→ 渠道三元组缺 → 30703（log 4）
// → ParseTkl（失败 → 降级 D，log 2）→ ItemDesc/ItemInfo 容错
// → rates:{itemID}:{pid} 缓存 → RatesURL（失败 → 降级 D）
// → tpwd:{clickURL} 缓存 → TpwdCreate（失败 → 降级 D）
// → Render → ConvertResult{degraded:false}，log 1（endpoint=convert）。
// host 恒传 ""：HTTP Host 提取属控制器职责，host 渠道匹配本期仅 partner+默认行生效
// （已知限制，见任务报告）。每条路径恰落一条 ConversionLog。
func (s *ConversionService) Convert(ctx context.Context, password, partner, clientIP string) (ConvertResult, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，恒钉 tid=0
	start := time.Now()

	tkl, ok := Extract(password)
	if !ok {
		s.appendLog(ctx, endpointConvert, partner, password, "", "", logStatusNoTkl, nil, clientIP, start)
		return ConvertResult{}, errs.New(errs.CodeTaoKoulingPasswordNotFound, nil)
	}

	c := s.cfg.Taokouling
	if c.Zhetaoke.Appkey == "" || c.Top.Appkey == "" || c.Top.Secret == "" {
		s.appendLog(ctx, endpointConvert, partner, password, tkl, "", logStatusHardErr, nil, clientIP, start)
		return ConvertResult{}, errs.New(errs.CodeTaoKoulingConfigMissing, nil)
	}

	// 渠道解析：host 恒 ""（见方法注释）；错误原样上抛。
	ch, err := Resolve(ctx, s.channels, partner, "")
	if err != nil {
		s.appendLog(ctx, endpointConvert, partner, password, tkl, "", logStatusHardErr, err, clientIP, start)
		return ConvertResult{}, err
	}
	// 日志 partner 优先取解析到的渠道短码。
	logPartner := ch.Partner
	if ch.PID == "" || ch.APIKey == "" || ch.TBName == "" {
		s.appendLog(ctx, endpointConvert, logPartner, password, tkl, "", logStatusHardErr, nil, clientIP, start)
		return ConvertResult{}, errs.New(errs.CodeTaoKoulingChannelMissing, nil)
	}

	itemID, pErr := s.parser.ParseTkl(ctx, tkl)
	if pErr != nil {
		// 解析未成 → ItemID 恒空（降级结果只带已解析的 ID）
		return s.degrade(ctx, password, logPartner, tkl, "", clientIP, start, pErr), nil
	}

	// ItemDesc 容错：失败 → reason=""（理由行省略，不落日志）。R7：apikey 按渠道传。
	reason, _ := s.rates.ItemDesc(ctx, itemID, ch.APIKey)

	// ItemInfo 容错：失败 → TitleOf(input) 兜底（miss → ""），价格行省略。
	var item client.Item
	var title string
	prices := RenderPrices{}
	if it, iErr := s.top.ItemInfo(ctx, itemID, clientIP); iErr == nil {
		item = it
		title = item.Title
		prices = RenderPrices{Available: true, Zk: item.ZkPrice, Reserve: item.ReservePrice}
	} else if t, ok := TitleOf(password); ok {
		title = t
	}

	// 缓存 TTL：CacheTTLSeconds<=0 → 1800s。
	ttl := time.Duration(c.CacheTTLSeconds) * time.Second
	if c.CacheTTLSeconds <= 0 {
		ttl = defaultCacheTTL
	}
	cacheOn := c.CacheEnabled && s.cache != nil

	// rates 缓存：命中跳过 RatesURL。
	var clickURL string
	var coupon int
	ratesHit := false
	ratesKey := fmt.Sprintf("rates:%s:%s", itemID, ch.PID)
	if cacheOn {
		if v, ok := s.cache.Get(ratesKey); ok {
			if e, ok := v.(ratesEntry); ok {
				clickURL, coupon, ratesHit = e.ClickURL, e.Coupon, true
			}
		}
	}
	if !ratesHit {
		u, cp, rErr := s.rates.RatesURL(ctx, ch.PID, ch.APIKey, ch.TBName, itemID)
		if rErr != nil {
			return s.degrade(ctx, password, logPartner, tkl, itemID, clientIP, start, rErr), nil
		}
		clickURL, coupon = u, cp
		if cacheOn {
			s.cache.Set(ratesKey, ratesEntry{ClickURL: u, Coupon: cp}, ttl)
		}
	}

	// tpwd 缓存：命中跳过 TpwdCreate。
	var newTkl string
	tPwdHit := false
	tpwdKey := fmt.Sprintf("tpwd:%s", clickURL)
	if cacheOn {
		if v, ok := s.cache.Get(tpwdKey); ok {
			if s2, ok := v.(string); ok {
				newTkl, tPwdHit = s2, true
			}
		}
	}
	if !tPwdHit {
		n, tErr := s.top.TpwdCreate(ctx, title, clickURL, item.PictURL)
		if tErr != nil {
			return s.degrade(ctx, password, logPartner, tkl, itemID, clientIP, start, tErr), nil
		}
		newTkl = n
		if cacheOn {
			s.cache.Set(tpwdKey, n, ttl)
		}
	}

	text := Render(title, prices, coupon, reason, newTkl, RenderConfig{
		RebateRate:  c.RebateRate, // Render 内 <=0 归一 0.05
		TutorialURL: c.RebateTutorialURL,
	})
	s.appendLog(ctx, endpointConvert, logPartner, password, tkl, itemID, logStatusOK, nil, clientIP, start)
	return ConvertResult{
		Text:        text,
		ItemID:      itemID,
		Degraded:    false,
		Title:       title,
		CouponMoney: coupon,
		NewTkl:      newTkl,
	}, nil
}

// degrade 降级路径 D：FallbackSearchURL 非空 → 「点击搜索其它店铺{短链或长链}」
// （长链 = fallback + url.QueryEscape(TitleOf(input))；TitleOf miss → 仅 fallback；
// Shorten 失败/空串回退长链）；为空 → 纯提示文案。
// 落一条 status=2 日志后返回 Degraded 结果，永不返回 error。
func (s *ConversionService) degrade(ctx context.Context, password, partner, extracted, itemID, clientIP string, start time.Time, cause error) ConvertResult {
	text := "未识别到有效淘口令，请检查后重试"
	if fb := s.cfg.Taokouling.FallbackSearchURL; fb != "" {
		link := fb
		if title, ok := TitleOf(password); ok {
			link = fb + url.QueryEscape(title)
		}
		if s.short != nil {
			if short, err := s.short.Shorten(ctx, link); err == nil && short != "" {
				link = short
			}
		}
		text = "「点击搜索其它店铺" + link + "」"
	}
	s.appendLog(ctx, endpointConvert, partner, password, extracted, itemID, logStatusDegraded, cause, clientIP, start)
	return ConvertResult{Text: text, ItemID: itemID, Degraded: true}
}

// appendLog 落一条转换日志：input_text 截 500、error 截 255、duration_ms 取实测耗时；
// TenantID 由仓储恒钉 0。Append 失败只 warn（logger 未初始化则吞掉），绝不影响响应。
func (s *ConversionService) appendLog(ctx context.Context, endpoint, partner, password, extracted, itemID string, status int8, cause error, clientIP string, start time.Time) {
	if s.logs == nil {
		return
	}
	rec := &model.ConversionLog{
		Endpoint:     endpoint,
		Partner:      partner,
		InputText:    trunc(password, 500),
		ExtractedTkl: extracted,
		ItemID:       itemID,
		Status:       status,
		DurationMs:   int(time.Since(start).Milliseconds()),
		IP:           clientIP,
	}
	if cause != nil {
		rec.Error = trunc(cause.Error(), 255)
	}
	if err := s.logs.Append(ctx, rec); err != nil && logger.Log != nil {
		logger.Log.Warn("taokouling: conversion log append failed: " + err.Error())
	}
}

// trunc 按字符数截断（DB varchar 按字符计），避免截断多字节 UTF-8。
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
