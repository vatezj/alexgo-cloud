package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"alexGo-cloud/modules/oauth2/model"
	"alexGo-cloud/modules/oauth2/repository"
	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// newProtoRouter 搭 /api 组下的协议端点测试路由（Task 9/11/13 共用夹具）。
// cfg nil → 默认 LoginPage=/auth/login。返回 sqlite 上的真实 Clients/Codes 供种子。
func newProtoRouter(t *testing.T, cfg *config.Config) (*gin.Engine, *oauth2svc.ClientService, *oauth2svc.AuthorizeCodeService) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Oauth2Client{}, &model.Oauth2AuthorizeCode{}); err != nil {
		t.Fatal(err)
	}
	if cfg == nil {
		cfg = &config.Config{}
		cfg.Auth.LoginPage = "/auth/login"
	}
	clients := oauth2svc.NewClientService(repository.NewClientRepository(db))
	codes := oauth2svc.NewAuthorizeCodeService(repository.NewAuthorizeCodeRepository(db))
	proto := NewProtocolController(ProtocolDeps{
		Captcha: oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg}),
		Codes:   codes,
		Clients: clients,
		Cfg:     cfg,
	})
	r := gin.New()
	g := r.Group("/api")
	proto.Register(g)
	return r, clients, codes
}

func seedSpaClient(t *testing.T, clients *oauth2svc.ClientService) {
	t.Helper()
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "spa", ClientName: "SPA",
		GrantTypes:    "authorization_code,password,refresh_token",
		RedirectURIs:  "http://localhost:5173/callback",
		AccountTypes:  "system",
	}); err != nil {
		t.Fatal(err)
	}
}

// Task 3 captcha 端点回归（迁到新夹具）。
func TestCaptchaImage_OK(t *testing.T) {
	r, _, _ := newProtoRouter(t, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth2/captcha", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "uuid") || !strings.Contains(body, "image") {
		t.Errorf("captcha 响应缺字段: %s", body)
	}
}

// Task 3 captcha 开关回归（迁到新夹具，断言保留）：开关关时端点仍 200 + enabled=false。
func TestCaptchaImage_DisabledStillOK(t *testing.T) {
	cfg := &config.Config{} // Captcha.Enabled 零值 false
	r, _, _ := newProtoRouter(t, cfg)
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

func TestAuthorize_OK_302LoginPage(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	seedSpaClient(t, clients)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=spa&response_type=code&state=xyz&scope=all&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/auth/login?") {
		t.Errorf("Location = %q, 应指登录页", loc)
	}
	for _, want := range []string{"client_id=spa", "state=xyz", "response_type=code"} {
		if !strings.Contains(loc, want) {
			t.Errorf("Location 缺回带参数 %q: %q", want, loc)
		}
	}
}

// Review Focus #1：redirect_uri 篡改 → 400 且绝不带 Location。
func TestAuthorize_Validate_RedirectMismatch_No302(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	seedSpaClient(t, clients)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=spa&response_type=code&redirect_uri="+
			url.QueryEscape("http://evil.example/steal"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("篡改 redirect 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), `"error":"invalid_request"`) {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestAuthorize_UnknownClient_400(t *testing.T) {
	r, _, _ := newProtoRouter(t, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=nope&response_type=code&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_client") {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("不得带 Location, got %q", loc)
	}
}

func TestGrant_NotAllowed_400(t *testing.T) {
	r, clients, _ := newProtoRouter(t, nil)
	if err := clients.Create(context.Background(), &model.Oauth2Client{
		ClientID: "pw-only", GrantTypes: "password",
		RedirectURIs: "http://localhost:5173/callback", AccountTypes: "system",
	}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/oauth2/authorize?client_id=pw-only&response_type=code&redirect_uri="+
			url.QueryEscape("http://localhost:5173/callback"), nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unauthorized_client") {
		t.Errorf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("不得带 Location, got %q", loc)
	}
}
