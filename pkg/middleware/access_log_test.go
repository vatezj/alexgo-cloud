package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/config"
)

// accessEntryFields 摘取断言所需字段（string 走 String，数值走 Integer）。
func accessEntryFields(e observer.LoggedEntry) (method, path string, status int64) {
	method, path, status = "", "", -1
	for _, f := range e.Context {
		switch f.Key {
		case "method":
			method = f.String
		case "path":
			path = f.String
		case "status":
			status = f.Integer
		}
	}
	return
}

func TestAccessLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(func(c *gin.Context) { // fake Auth：写入 claims
		c.Set("claims", &auth.Claims{UserID: 7, TenantID: 3})
	})
	r.Use(AccessLog())
	r.GET("/api/admin/test", func(c *gin.Context) { c.Status(200) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/test?x=1", nil)
	r.ServeHTTP(w, req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	fields := map[string]any{}
	for _, f := range entries[0].Context {
		// zap.Uint64/zap.Int 等数值字段的值在 Integer（String 为空），
		// 统一转十进制字符串后与断言比较。
		if f.Type == zapcore.StringType {
			fields[f.Key] = f.String
		} else {
			fields[f.Key] = strconv.FormatInt(f.Integer, 10)
		}
	}
	if fields["method"] != "GET" || fields["path"] != "/api/admin/test" {
		t.Fatalf("fields = %v", fields)
	}
	if fields["user_id"] != "7" || fields["tenant_id"] != "3" {
		t.Fatalf("user/tenant missing: %v", fields)
	}
	if fields["status"] != "200" {
		t.Fatalf("status = %v", fields["status"])
	}
}

func TestAccessLogLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.DebugLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(AccessLog())
	r.GET("/err", func(c *gin.Context) { c.Status(500) })
	r.GET("/bad", func(c *gin.Context) { c.Status(404) })
	r.GET("/health", func(c *gin.Context) { c.Status(200) })

	for _, path := range []string{"/err", "/bad", "/health"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	}
	if len(logs.All()) != 2 { // /health 被排除
		t.Fatalf("entries = %d, want 2", len(logs.All()))
	}
	levels := map[string]zapcore.Level{}
	for _, e := range logs.All() {
		p, _ := e.ContextMap()["path"].(string)
		levels[p] = e.Level
	}
	if levels["/err"] != zapcore.ErrorLevel || levels["/bad"] != zapcore.WarnLevel {
		t.Fatalf("levels = %v", levels)
	}
}

// 终审修复②（方案②）：Auth 的 401 中止短路了链尾 AccessLog，须在 Abort 点
// 补记——恰好一条访问日志、status 401、级别 Warn；401 无 claims → 不得有 user_id。
func TestAccessLogOnAuthAbort401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	r := gin.New()
	r.Use(NewAuthMiddleware(AuthDeps{})) // 无 Cfg/Validator → parseClaims 必失败 → 401
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/system/users", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (aborted request logs once)", len(entries))
	}
	if entries[0].Level != zapcore.WarnLevel {
		t.Fatalf("level = %v, want Warn (4xx)", entries[0].Level)
	}
	method, path, status := accessEntryFields(entries[0])
	if method != "GET" || path != "/api/admin/system/users" || status != 401 {
		t.Fatalf("method/path/status = %q/%q/%d, want GET/…/401", method, path, status)
	}
	for _, f := range entries[0].Context {
		if f.Key == "user_id" {
			t.Fatalf("401 has no claims — user_id must be absent, got %v", f)
		}
	}
}

// 终审修复②（方案②）：熔断 Open 的 503 中止同样补记一条访问日志（级别 Error）。
func TestAccessLogOnBreakerOpen503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	old := loggerForTest
	loggerForTest = zap.New(core)
	t.Cleanup(func() { loggerForTest = old })

	cfg := &config.Config{}
	cfg.Breaker.Enabled = true
	cb := circuitbreaker.NewCircuitBreaker(1, time.Minute)
	if err := cb.Do(func() error { return errors.New("down") }); err == nil {
		t.Fatal("setup: breaker should open")
	}

	r := gin.New()
	r.Use(RateLimitAndBreaker(cfg, nil, cb))
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/system/users", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1 (aborted request logs once)", len(entries))
	}
	if entries[0].Level != zapcore.ErrorLevel {
		t.Fatalf("level = %v, want Error (5xx)", entries[0].Level)
	}
	method, path, status := accessEntryFields(entries[0])
	if method != "GET" || path != "/api/admin/system/users" || status != 503 {
		t.Fatalf("method/path/status = %q/%q/%d, want GET/…/503", method, path, status)
	}
}
