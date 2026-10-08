package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/tenant"
)

// run 注入中间件并执行一个把最终 tenant_id 写回的探针 handler。
func run(t *testing.T, lookup tenant.DomainLookup, headerValue, host string) uint64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var got uint64
	r.Use(NewTenantMiddleware(lookup))
	r.GET("/", func(c *gin.Context) {
		got = tenant.TenantIDFromContext(c.Request.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if headerValue != "" {
		req.Header.Set("X-Tenant-ID", headerValue)
	}
	req.Host = host
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	return got
}

// 显式 Header 优先于域名解析。
func TestTenantMiddleware_HeaderWinsOverLookup(t *testing.T) {
	called := false
	lookup := func(context.Context, string) (uint64, error) {
		called = true
		return 42, nil
	}
	if got := run(t, lookup, "7", "a.example.com"); got != 7 {
		t.Fatalf("got %d, want 7", got)
	}
	if called {
		t.Fatal("lookup must not be called when X-Tenant-ID header is present")
	}
}

// Header 缺失/非法 → Host 去端口后交给域名解析。
func TestTenantMiddleware_HostPortStripped(t *testing.T) {
	var gotHost string
	lookup := func(_ context.Context, host string) (uint64, error) {
		gotHost = host
		return 9, nil
	}
	if got := run(t, lookup, "", "b.example.com:8080"); got != 9 {
		t.Fatalf("got %d, want 9", got)
	}
	if gotHost != "b.example.com" {
		t.Fatalf("lookup host = %q, want port stripped", gotHost)
	}
}

// lookup 为 nil（未装配域名解析）→ 只认 Header，tid=0，不 panic。
func TestTenantMiddleware_NilLookupSafe(t *testing.T) {
	if got := run(t, nil, "", "c.example.com"); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	if got := run(t, nil, "3", "c.example.com"); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}

// 解析失败按未解析处理：tid=0，不阻断请求。
func TestTenantMiddleware_LookupErrorFallsBackToZero(t *testing.T) {
	lookup := func(context.Context, string) (uint64, error) {
		return 0, context.Canceled
	}
	if got := run(t, lookup, "", "d.example.com"); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}
