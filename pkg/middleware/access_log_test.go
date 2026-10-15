package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"alexGo-cloud/pkg/auth"
)

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
