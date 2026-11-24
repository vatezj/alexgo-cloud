package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alexGo-cloud/modules/taokouling/client"
	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/config"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
)

// ---------- fakes ----------

// fakeTklParser ParseTkl 桩。
type fakeTklParser struct {
	itemID string
	err    error
	calls  int
	tkls   []string
}

func (f *fakeTklParser) ParseTkl(_ context.Context, tkl string) (string, error) {
	f.calls++
	f.tkls = append(f.tkls, tkl)
	if f.err != nil {
		return "", f.err // 与真实 client 一致：失败恒返空 itemID
	}
	return f.itemID, nil
}

// fakeRatesClient RatesURL/ItemDesc 桩；记录入参供断言（R7 渠道 apikey）。
type fakeRatesClient struct {
	url        string
	coupon     int
	ratesErr   error
	desc       string
	descErr    error
	ratesCalls int
	descCalls  int
	lastPID    string
	lastAPIKey string
	lastTB     string
	lastItem   string
	descItem   string
	descKey    string
}

func (f *fakeRatesClient) RatesURL(_ context.Context, pid, apikey, tbName, itemID string) (string, int, error) {
	f.ratesCalls++
	f.lastPID, f.lastAPIKey, f.lastTB, f.lastItem = pid, apikey, tbName, itemID
	return f.url, f.coupon, f.ratesErr
}

func (f *fakeRatesClient) ItemDesc(_ context.Context, itemID, apikey string) (string, error) {
	f.descCalls++
	f.descItem, f.descKey = itemID, apikey
	return f.desc, f.descErr
}

// fakeTopClient TpwdCreate/ItemInfo 桩。
type fakeTopClient struct {
	item      client.Item
	itemErr   error
	tpwd      string
	tpwdErr   error
	tpwdCalls int
	itemCalls int
	tpwdText  string
	tpwdURL   string
	tpwdLogo  string
}

func (f *fakeTopClient) TpwdCreate(_ context.Context, text, url, logo string) (string, error) {
	f.tpwdCalls++
	f.tpwdText, f.tpwdURL, f.tpwdLogo = text, url, logo
	return f.tpwd, f.tpwdErr
}

func (f *fakeTopClient) ItemInfo(_ context.Context, _, _ string) (client.Item, error) {
	f.itemCalls++
	return f.item, f.itemErr
}

// fakeShortener Shorten 桩。
type fakeShortener struct {
	short string
	err   error
	calls int
	longs []string
}

func (f *fakeShortener) Shorten(_ context.Context, longURL string) (string, error) {
	f.calls++
	f.longs = append(f.longs, longURL)
	return f.short, f.err
}

// fakeConvertLogs LogStore 桩：记录全部 rec。
type fakeConvertLogs struct {
	recs []*model.ConversionLog
	err  error
}

func (f *fakeConvertLogs) Append(_ context.Context, rec *model.ConversionLog) error {
	f.recs = append(f.recs, rec)
	return f.err
}

// cvtChannels ChannelStore 桩：记录收到的 ctx（供 tid=0 断言）。
type cvtChannels struct {
	partnerCh *model.Channel
	hostCh    *model.Channel
	defaultCh *model.Channel
	seenCtxs  []context.Context
}

func (f *cvtChannels) GetEnabledByPartner(ctx context.Context, _ string) (*model.Channel, error) {
	f.seenCtxs = append(f.seenCtxs, ctx)
	return f.partnerCh, nil
}

func (f *cvtChannels) GetEnabledByHost(ctx context.Context, _ string) (*model.Channel, error) {
	f.seenCtxs = append(f.seenCtxs, ctx)
	return f.hostCh, nil
}

func (f *cvtChannels) GetDefault(ctx context.Context) (*model.Channel, error) {
	f.seenCtxs = append(f.seenCtxs, ctx)
	return f.defaultCh, nil
}

// ---------- harness ----------

const (
	samplePassword = "【某爆款好物】复制这段描述￥a1b2c3d4e5￥后到淘宆"
	sampleTkl      = "a1b2c3d4e5"
)

func mkTestChannel() *model.Channel {
	return &model.Channel{
		Name: "默认渠道", Partner: "", Host: "",
		PID: "mm_123_456_789", APIKey: "hd-apikey-1", TBName: "tb_nickname", Status: 1,
	}
}

func mkTestCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Taokouling.Zhetaoke.Appkey = "zk-appkey"
	cfg.Taokouling.Top.Appkey = "top-appkey"
	cfg.Taokouling.Top.Secret = "top-secret"
	return cfg
}

// cvtHarness 每例一副桩 + 直构 config + 假钟 TTLCache。
type cvtHarness struct {
	svc    *ConversionService
	parser *fakeTklParser
	rates  *fakeRatesClient
	top    *fakeTopClient
	short  *fakeShortener
	logs   *fakeConvertLogs
	chans  *cvtChannels
	cfg    *config.Config
	cache  *TTLCache
}

func newCvtHarness() *cvtHarness {
	h := &cvtHarness{
		parser: &fakeTklParser{itemID: "123456789"},
		rates: &fakeRatesClient{
			url: "https://s.click.example/abc", coupon: 30, desc: "质量很好",
		},
		top: &fakeTopClient{
			item: client.Item{
				Title: "某商品标题", PictURL: "https://img.example/p.jpg",
				ZkPrice: 99.9, ReservePrice: 129.9,
			},
			tpwd: "￥new￥",
		},
		short: &fakeShortener{},
		logs:  &fakeConvertLogs{},
		chans: &cvtChannels{defaultCh: mkTestChannel()},
		cfg:   mkTestCfg(),
	}
	clk := &fakeClock{t: time.Unix(1700000000, 0)} // 假钟，见 cache_test.go
	h.cache = NewTTLCache(clk.Now)
	h.svc = NewConversionService(ConvertParams{
		Parser: h.parser, Rates: h.rates, Top: h.top, Short: h.short,
		Channels: h.chans, Logs: h.logs, Cfg: h.cfg, Cache: h.cache,
	})
	return h
}

// onlyLog 断言恰落一条日志并返回之（每条失败/成功路径恰一条）。
func onlyLog(t *testing.T, logs *fakeConvertLogs) *model.ConversionLog {
	t.Helper()
	if len(logs.recs) != 1 {
		t.Fatalf("log rows = %d, want exactly 1", len(logs.recs))
	}
	return logs.recs[0]
}

// ---------- Convert ----------

func TestConvert_Success_Coupon(t *testing.T) {
	h := newCvtHarness()

	res, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Convert err=%v, want nil", err)
	}
	if res.Degraded {
		t.Fatal("Degraded=true, want false")
	}
	if !strings.Contains(res.Text, "【券后价】69.90元") {
		t.Errorf("text 缺【券后价】69.90元（99.9-30）:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "￥new￥") {
		t.Errorf("text 缺新口令 ￥new￥:\n%s", res.Text)
	}
	if res.CouponMoney != 30 {
		t.Errorf("CouponMoney=%d, want 30", res.CouponMoney)
	}
	if res.ItemID != "123456789" {
		t.Errorf("ItemID=%q, want 123456789", res.ItemID)
	}
	if res.NewTkl != "￥new￥" {
		t.Errorf("NewTkl=%q, want ￥new￥", res.NewTkl)
	}
	if res.Title != "某商品标题" {
		t.Errorf("Title=%q, want 某商品标题", res.Title)
	}
	// R7：ItemDesc 用解析到的渠道 apikey，非硬编码。
	if h.rates.descKey != "hd-apikey-1" {
		t.Errorf("ItemDesc apikey=%q, want 渠道 hd-apikey-1", h.rates.descKey)
	}
	// RatesURL 按渠道三元组调用。
	if h.rates.lastPID != "mm_123_456_789" || h.rates.lastTB != "tb_nickname" {
		t.Errorf("RatesURL 入参 pid=%q tb=%q, want 渠道三元组", h.rates.lastPID, h.rates.lastTB)
	}
	// TpwdCreate(title, clickURL, pict)
	if h.top.tpwdText != "某商品标题" || h.top.tpwdURL != "https://s.click.example/abc" ||
		h.top.tpwdLogo != "https://img.example/p.jpg" {
		t.Errorf("TpwdCreate 入参 text=%q url=%q logo=%q",
			h.top.tpwdText, h.top.tpwdURL, h.top.tpwdLogo)
	}

	rec := onlyLog(t, h.logs)
	if rec.Status != 1 || rec.Endpoint != "convert" {
		t.Errorf("log status=%d endpoint=%q, want 1/convert", rec.Status, rec.Endpoint)
	}
	if rec.ItemID != "123456789" || rec.InputText != samplePassword || rec.ExtractedTkl != sampleTkl {
		t.Errorf("log item=%q input=%q tkl=%q", rec.ItemID, rec.InputText, rec.ExtractedTkl)
	}
	if rec.IP != "1.2.3.4" {
		t.Errorf("log ip=%q, want 1.2.3.4", rec.IP)
	}
	if rec.Error != "" {
		t.Errorf("成功日志 error=%q, want 空", rec.Error)
	}
}

func TestConvert_Success_NoCoupon(t *testing.T) {
	h := newCvtHarness()
	h.rates.coupon = 0

	res, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Convert err=%v, want nil", err)
	}
	if res.Degraded {
		t.Fatal("Degraded=true, want false")
	}
	if !strings.Contains(res.Text, "【促销价】") {
		t.Errorf("text 缺【促销价】:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "限时特惠") {
		t.Errorf("text 缺 限时特惠:\n%s", res.Text)
	}
	if res.CouponMoney != 0 {
		t.Errorf("CouponMoney=%d, want 0", res.CouponMoney)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 1 || rec.Endpoint != "convert" {
		t.Errorf("log status=%d endpoint=%q, want 1/convert", rec.Status, rec.Endpoint)
	}
}

func TestConvert_ParseFail_Degrades(t *testing.T) {
	h := newCvtHarness()
	h.parser.err = client.ErrParseFailed

	res, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("降级不应返回 err, got %v", err)
	}
	if !res.Degraded {
		t.Fatal("Degraded=false, want true")
	}
	if res.Text == "" {
		t.Fatal("降级 text 为空")
	}
	if res.ItemID != "" {
		t.Errorf("解析失败 ItemID=%q, want 空", res.ItemID)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 2 {
		t.Errorf("log status=%d, want 2", rec.Status)
	}
	if rec.Error == "" {
		t.Error("降级日志 error 为空, want 非空原因")
	}
}

func TestConvert_RatesFail_Degrades(t *testing.T) {
	h := newCvtHarness()
	h.rates.ratesErr = client.ErrNoRates

	res, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("降级不应返回 err, got %v", err)
	}
	if !res.Degraded {
		t.Fatal("Degraded=false, want true")
	}
	if res.Text == "" {
		t.Fatal("降级 text 为空")
	}
	if res.ItemID != "123456789" {
		t.Errorf("已解析后降级 ItemID=%q, want 123456789", res.ItemID)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 2 {
		t.Errorf("log status=%d, want 2", rec.Status)
	}
	if rec.Error == "" {
		t.Error("降级日志 error 为空, want 非空原因")
	}
}

func TestConvert_TpwdFail_Degrades(t *testing.T) {
	h := newCvtHarness()
	h.top.tpwdErr = errors.New("tpwd create failed")

	res, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("降级不应返回 err, got %v", err)
	}
	if !res.Degraded {
		t.Fatal("Degraded=false, want true")
	}
	if res.Text == "" {
		t.Fatal("降级 text 为空")
	}
	if res.ItemID != "123456789" {
		t.Errorf("已解析后降级 ItemID=%q, want 123456789", res.ItemID)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 2 {
		t.Errorf("log status=%d, want 2", rec.Status)
	}
	if rec.Error == "" {
		t.Error("降级日志 error 为空, want 非空原因")
	}
}

func TestConvert_FallbackURLUsedWhenConfigured(t *testing.T) {
	h := newCvtHarness()
	h.cfg.Taokouling.FallbackSearchURL = "https://s.example/search?keyword="
	h.short.short = "http://s/xyz"
	h.rates.ratesErr = client.ErrNoRates // 触发降级 D
	pw := "【标题商品】复制打开淘宝￥a1b2c3d4e5￥"

	res, err := h.svc.Convert(context.Background(), pw, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("降级不应返回 err, got %v", err)
	}
	if !res.Degraded {
		t.Fatal("Degraded=false, want true")
	}
	if !strings.Contains(res.Text, "http://s/xyz") {
		t.Errorf("text 缺短链 http://s/xyz:\n%s", res.Text)
	}
	if h.short.calls != 1 {
		t.Errorf("Shorten calls=%d, want 1", h.short.calls)
	}
	// 长链 = fallback + QueryEscape("标题商品")
	wantLong := "https://s.example/search?keyword=%E6%A0%87%E9%A2%98%E5%95%86%E5%93%81"
	if len(h.short.longs) != 1 || h.short.longs[0] != wantLong {
		t.Errorf("Shorten 入参=%v, want [%s]", h.short.longs, wantLong)
	}
}

func TestConvert_NoPassword_30701(t *testing.T) {
	h := newCvtHarness()

	_, err := h.svc.Convert(context.Background(), "没有口令的普通文本", "", "1.2.3.4")
	if !errs.Is(err, errs.CodeTaoKoulingPasswordNotFound) {
		t.Fatalf("err=%v, want errs.Is(..., CodeTaoKoulingPasswordNotFound=30701)", err)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 3 || rec.Endpoint != "convert" {
		t.Errorf("log status=%d endpoint=%q, want 3/convert", rec.Status, rec.Endpoint)
	}
}

func TestConvert_NoDefaultChannel_30703(t *testing.T) {
	h := newCvtHarness()
	h.chans.defaultCh = nil // 三级全 miss

	_, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if !errs.Is(err, errs.CodeTaoKoulingChannelMissing) {
		t.Fatalf("err=%v, want errs.Is(..., CodeTaoKoulingChannelMissing=30703)", err)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 4 || rec.Endpoint != "convert" {
		t.Errorf("log status=%d endpoint=%q, want 4/convert", rec.Status, rec.Endpoint)
	}
}

func TestConvert_MissingTopSecret_30702(t *testing.T) {
	h := newCvtHarness()
	h.cfg.Taokouling.Top.Secret = ""

	_, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4")
	if !errs.Is(err, errs.CodeTaoKoulingConfigMissing) {
		t.Fatalf("err=%v, want errs.Is(..., CodeTaoKoulingConfigMissing=30702)", err)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 4 || rec.Endpoint != "convert" {
		t.Errorf("log status=%d endpoint=%q, want 4/convert", rec.Status, rec.Endpoint)
	}
}

func TestConvert_CacheHitSkipsSecondCalls(t *testing.T) {
	h := newCvtHarness()
	h.cfg.Taokouling.CacheEnabled = true

	if _, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4"); err != nil {
		t.Fatalf("first Convert err=%v, want nil", err)
	}
	if _, err := h.svc.Convert(context.Background(), samplePassword, "", "1.2.3.4"); err != nil {
		t.Fatalf("second Convert err=%v, want nil", err)
	}
	if h.rates.ratesCalls != 1 {
		t.Errorf("RatesURL calls=%d, want 1（缓存命中跳过）", h.rates.ratesCalls)
	}
	if h.top.tpwdCalls != 1 {
		t.Errorf("TpwdCreate calls=%d, want 1（缓存命中跳过）", h.top.tpwdCalls)
	}
	if len(h.logs.recs) != 2 {
		t.Errorf("log rows=%d, want 2（每调一条）", len(h.logs.recs))
	}
}

func TestConvert_SpoofTenantStillResolvesChannel(t *testing.T) {
	h := newCvtHarness()
	ctx := tenant.WithTenantID(context.Background(), 999) // 伪造租户

	res, err := h.svc.Convert(ctx, samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Convert err=%v, want nil", err)
	}
	if res.Degraded {
		t.Fatal("Degraded=true, want false")
	}
	if len(h.chans.seenCtxs) == 0 {
		t.Fatal("channel store 未被查询")
	}
	for i, c := range h.chans.seenCtxs {
		if got := tenant.TenantIDFromContext(c); got != 0 {
			t.Errorf("seenCtxs[%d] tid=%d, want 0（入口重包钉 0）", i, got)
		}
	}
}

// ---------- Parse ----------

func TestParse_Success(t *testing.T) {
	h := newCvtHarness()

	itemID, err := h.svc.Parse(context.Background(), samplePassword, "", "1.2.3.4")
	if err != nil {
		t.Fatalf("Parse err=%v, want nil", err)
	}
	if itemID != "123456789" {
		t.Errorf("itemID=%q, want 123456789", itemID)
	}
	if h.parser.calls != 1 || len(h.parser.tkls) != 1 || h.parser.tkls[0] != sampleTkl {
		t.Errorf("ParseTkl calls=%d tkls=%v, want 1/[ %s ]", h.parser.calls, h.parser.tkls, sampleTkl)
	}
	if len(h.chans.seenCtxs) != 0 {
		t.Errorf("轻量链路不应查渠道, seen=%d", len(h.chans.seenCtxs))
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 1 || rec.Endpoint != "parse" {
		t.Errorf("log status=%d endpoint=%q, want 1/parse", rec.Status, rec.Endpoint)
	}
	if rec.ItemID != "123456789" || rec.ExtractedTkl != sampleTkl {
		t.Errorf("log item=%q tkl=%q", rec.ItemID, rec.ExtractedTkl)
	}
}

func TestParse_Fail_30705(t *testing.T) {
	h := newCvtHarness()
	h.parser.err = client.ErrParseFailed

	_, err := h.svc.Parse(context.Background(), samplePassword, "", "1.2.3.4")
	if !errs.Is(err, errs.CodeTaoKoulingParseFailed) {
		t.Fatalf("err=%v, want errs.Is(..., CodeTaoKoulingParseFailed=30705)", err)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 4 || rec.Endpoint != "parse" {
		t.Errorf("log status=%d endpoint=%q, want 4/parse", rec.Status, rec.Endpoint)
	}
	if rec.Error == "" {
		t.Error("硬错日志 error 为空, want 非空原因")
	}
}

func TestParse_NoPassword_30701(t *testing.T) {
	h := newCvtHarness()

	_, err := h.svc.Parse(context.Background(), "无口令文本", "", "1.2.3.4")
	if !errs.Is(err, errs.CodeTaoKoulingPasswordNotFound) {
		t.Fatalf("err=%v, want errs.Is(..., CodeTaoKoulingPasswordNotFound=30701)", err)
	}
	rec := onlyLog(t, h.logs)
	if rec.Status != 3 || rec.Endpoint != "parse" {
		t.Errorf("log status=%d endpoint=%q, want 3/parse", rec.Status, rec.Endpoint)
	}
}
