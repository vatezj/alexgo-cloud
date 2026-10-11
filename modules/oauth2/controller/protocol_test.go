package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// TestCaptchaImage_OK 无论开关 GET /oauth2/captcha 都 200 + uuid/image/enabled（spec §3.4）。
func TestCaptchaImage_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = true
	p := NewProtocolController(oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}))
	r := gin.New()
	p.Register(r.Group("/api"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth2/captcha", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var ch struct {
		UUID    string `json:"uuid"`
		Image   string `json:"image"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("响应非 JSON: %v (%s)", err, w.Body.String())
	}
	if ch.UUID == "" || ch.Image == "" {
		t.Fatalf("响应缺 uuid/image 字段: %s", w.Body.String())
	}
}

// TestCaptchaImage_DisabledStillOK 开关关时端点仍 200 + enabled=false（开关只影响登录校验）。
func TestCaptchaImage_DisabledStillOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	p := NewProtocolController(oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}))
	r := gin.New()
	p.Register(r.Group("/api"))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth2/captcha", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var ch struct {
		UUID    string `json:"uuid"`
		Image   string `json:"image"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ch); err != nil {
		t.Fatalf("响应非 JSON: %v (%s)", err, w.Body.String())
	}
	if ch.UUID == "" || ch.Image == "" {
		t.Fatalf("响应缺 uuid/image 字段: %s", w.Body.String())
	}
	if ch.Enabled {
		t.Errorf("enabled 应 false, body=%s", w.Body.String())
	}
}
