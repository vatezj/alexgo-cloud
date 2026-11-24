package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/taokouling/service"
	errs "alexGo-cloud/pkg/errors"
)

// fakeConverter Converter 测试桩：可配返回值并计数（限流用例断言调用不增）。
type fakeConverter struct {
	parseItem   string
	parseErr    error
	convertRes  service.ConvertResult
	convertErr  error
	parseCalls  int
	convertCall int
}

func (f *fakeConverter) Parse(_ context.Context, _, _, _ string) (string, error) {
	f.parseCalls++
	return f.parseItem, f.parseErr
}

func (f *fakeConverter) Convert(_ context.Context, _, _, _ string) (service.ConvertResult, error) {
	f.convertCall++
	return f.convertRes, f.convertErr
}

// newAppRouter 起 parse/convert 两路由（与 module.go RegisterRoutes app 段同形状）。
func newAppRouter(t *testing.T, svc Converter, limiter *service.RateLimiter) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAppController(svc, limiter)
	r.POST("/parse", c.Parse)
	r.POST("/convert", c.Convert)
	return r
}

// unlimitedLimiter 高限流窗口（不干扰除限流外的用例）。
func unlimitedLimiter() *service.RateLimiter {
	return service.NewRateLimiter(1<<30, time.Minute, time.Now)
}

// postJSON 发 POST 并解析 {code,msg,data} 信封。
func postJSON(t *testing.T, r *gin.Engine, path, body string) (int, int, json.RawMessage, string) {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(http.MethodPost, path, nil)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("信封解析失败: %v body=%s", err, w.Body.String())
	}
	return w.Code, env.Code, env.Data, env.Msg
}

// TestAppParse_SuccessEnvelope 桩返回 "123" → 200 {"code":0,"data":{"item_id":"123"}}（spec §4.1）。
func TestAppParse_SuccessEnvelope(t *testing.T) {
	fake := &fakeConverter{parseItem: "123"}
	r := newAppRouter(t, fake, unlimitedLimiter())

	status, code, data, _ := postJSON(t, r, "/parse", `{"password":"tkl"}`)
	if status != http.StatusOK || code != 0 {
		t.Fatalf("应 200/code0: status=%d code=%d", status, code)
	}
	var payload struct {
		ItemID string `json:"item_id"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.ItemID != "123" {
		t.Fatalf("data 应为 {\"item_id\":\"123\"}: data=%s err=%v", data, err)
	}
}

// TestAppParse_PasswordMissing_10001 空 body → 400 + code 10001。
func TestAppParse_PasswordMissing_10001(t *testing.T) {
	fake := &fakeConverter{parseItem: "123"}
	r := newAppRouter(t, fake, unlimitedLimiter())

	status, code, _, _ := postJSON(t, r, "/parse", ``)
	if status != http.StatusBadRequest || code != 10001 {
		t.Fatalf("应 400/10001: status=%d code=%d", status, code)
	}
	if fake.parseCalls != 0 {
		t.Fatalf("参数校验失败不应调 service: parseCalls=%d", fake.parseCalls)
	}
}

// TestAppConvert_DegradedStill200 桩返回 Degraded:true → 200 + data.degraded==true。
func TestAppConvert_DegradedStill200(t *testing.T) {
	fake := &fakeConverter{convertRes: service.ConvertResult{
		Text: "未识别到有效淘口令，请检查后重试", Degraded: true,
	}}
	r := newAppRouter(t, fake, unlimitedLimiter())

	status, code, data, _ := postJSON(t, r, "/convert", `{"password":"tkl"}`)
	if status != http.StatusOK || code != 0 {
		t.Fatalf("降级仍应 200/code0: status=%d code=%d", status, code)
	}
	var res struct {
		Degraded bool `json:"degraded"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		t.Fatalf("data 解析失败: %v body=%s", err, data)
	}
	if !res.Degraded {
		t.Fatalf("data.degraded 应为 true: %s", data)
	}
}

// TestAppParse_RateLimitedWrites429 limiter limit=1，连续两请求 → 第二个 429 + code==30704。
func TestAppParse_RateLimitedWrites429(t *testing.T) {
	fake := &fakeConverter{parseItem: "123"}
	limiter := service.NewRateLimiter(1, time.Minute, time.Now)
	r := newAppRouter(t, fake, limiter)

	status1, code1, _, _ := postJSON(t, r, "/parse", `{"password":"tkl"}`)
	if status1 != http.StatusOK || code1 != 0 {
		t.Fatalf("首请求应 200/code0: status=%d code=%d", status1, code1)
	}
	status2, code2, _, _ := postJSON(t, r, "/parse", `{"password":"tkl"}`)
	if status2 != http.StatusTooManyRequests {
		t.Fatalf("次请求应 429: status=%d", status2)
	}
	if code2 != int(errs.CodeTaoKoulingRateLimited) {
		t.Fatalf("429 code 应 30704: code=%d", code2)
	}
}

// TestAppParse_RateLimitedDoesNotLog 429 场景下 fake Converter 调用计数不因被限请求增加。
func TestAppParse_RateLimitedDoesNotLog(t *testing.T) {
	fake := &fakeConverter{parseItem: "123"}
	limiter := service.NewRateLimiter(1, time.Minute, time.Now)
	r := newAppRouter(t, fake, limiter)

	postJSON(t, r, "/parse", `{"password":"tkl"}`) // 消耗唯一配额
	afterFirst := fake.parseCalls
	status, _, _, _ := postJSON(t, r, "/parse", `{"password":"tkl"}`)
	if status != http.StatusTooManyRequests {
		t.Fatalf("应 429: status=%d", status)
	}
	if fake.parseCalls != afterFirst {
		t.Fatalf("限流请求不得触达 service: before=%d after=%d", afterFirst, fake.parseCalls)
	}
}

// TestAppParse_ErrorPassesThrough 桩返回 errs.New(CodeTaoKoulingParseFailed) → 400 + code 30705。
func TestAppParse_ErrorPassesThrough(t *testing.T) {
	fake := &fakeConverter{parseErr: errs.New(errs.CodeTaoKoulingParseFailed, nil)}
	r := newAppRouter(t, fake, unlimitedLimiter())

	status, code, data, _ := postJSON(t, r, "/parse", `{"password":"tkl"}`)
	if status != http.StatusBadRequest || code != int(errs.CodeTaoKoulingParseFailed) {
		t.Fatalf("应 400/30705: status=%d code=%d", status, code)
	}
	if string(data) != "null" {
		t.Fatalf("失败 data 应 null: %s", data)
	}
}
