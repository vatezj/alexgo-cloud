package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
)

// 熔断打开期间，健康/指标端点必须仍然可达（否则 DB 故障会经 liveness 503 触发全员重启）。
func TestRateLimitAndBreaker_HealthEndpointsExemptWhenBreakerOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Breaker.Enabled = true

	cb := circuitbreaker.NewCircuitBreaker(1, time.Minute)
	if err := cb.Do(func() error { return errors.New("down") }); err == nil {
		t.Fatal("setup: breaker should open")
	}
	// cb 现处 Open：任何经 Do 的请求都会被拒。

	r := gin.New()
	r.Use(middleware.RateLimitAndBreaker(cfg, nil, cb))
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/health/ready", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/metrics", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for _, path := range []string{"/health", "/health/ready", "/metrics"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != http.StatusOK {
			t.Errorf("%s: status = %d with breaker OPEN, want 200 (exempt)", path, w.Code)
		}
	}

	// 对照：非豁免路径在熔断打开时必须被拒（证明豁免是特例而非全局失效）。
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/admin/system/users", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("api path: status = %d, want 503 while breaker open", w.Code)
	}
}
