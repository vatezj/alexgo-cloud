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
	"alexGo-cloud/pkg/token"
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

// —— Task 11 夹具：真 UnifiedLogin + 真 TokenService 的协议全栈 ——

// stubUA UserAuthenticator 桩：记账调用——验证码/白名单拒绝时不得触达凭据校验（ua.calls==0 断言）。
type stubUA struct {
	ident *oauth2svc.AuthIdentity
	err   error
	calls []string
}

func (s *stubUA) Authenticate(_ context.Context, username, _ string) (*oauth2svc.AuthIdentity, error) {
	s.calls = append(s.calls, username)
	if s.err != nil {
		return nil, s.err
	}
	if s.ident != nil {
		return s.ident, nil
	}
	return &oauth2svc.AuthIdentity{UserID: 3, Username: username, AccountType: oauth2svc.AccountTypeSystem}, nil
}

// fakeIssuer token.Issuer 桩：Issue 恒出 at-1/rt-1，Refresh 出 at-2/rt-2。
type fakeIssuer struct{}

func (fakeIssuer) Issue(context.Context, token.IssueParams) (*token.Issued, error) {
	return &token.Issued{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200}, nil
}
func (fakeIssuer) Refresh(context.Context, string) (*token.Issued, error) {
	return &token.Issued{AccessToken: "at-2", RefreshToken: "rt-2", ExpiresIn: 7200}, nil
}
func (fakeIssuer) Revoke(context.Context, string) error                   { return nil }
func (fakeIssuer) RevokeAll(context.Context, token.UserType, uint64) error { return nil }

type fullEnv struct {
	clients *oauth2svc.ClientService
	codes   *oauth2svc.AuthorizeCodeService
	ua      *stubUA
}

// newFullRouter 搭真 UnifiedLogin + 真 TokenService 的协议全栈（cfg nil → 默认 LoginPage）。
// 与 newProtoRouter（Task 9，无 Login/Tokens）并存：老测试不动，新测试用本夹具。
func newFullRouter(t *testing.T, cfg *config.Config) (*gin.Engine, *fullEnv) {
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
	ua := &stubUA{}
	iss := fakeIssuer{}
	captcha := oauth2svc.NewCaptchaService(oauth2svc.CaptchaDeps{Cfg: cfg})
	ul := oauth2svc.NewUnifiedLogin(oauth2svc.UnifiedDeps{
		Cfg: cfg, Issuer: iss, Captcha: captcha, UserAuth: ua,
	})
	ts := oauth2svc.NewTokenService(oauth2svc.TokenServiceDeps{
		Cfg: cfg, Clients: clients, Codes: codes, Login: ul, Issuer: iss,
	})
	proto := NewProtocolController(ProtocolDeps{
		Captcha: captcha, Codes: codes, Clients: clients, Cfg: cfg,
		Login: ul, Tokens: ts,
	})
	r := gin.New()
	proto.Register(r.Group("/api"))
	return r, &fullEnv{clients: clients, codes: codes, ua: ua}
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Review Focus #1：authorize/login 的 redirect 篡改 → 400 且无 Location，
// 且拒绝必须发生在凭据校验之前（ua.calls==0）。
func TestAuthorizeLogin_RedirectTamper_400NoLocation(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://evil.example/steal","username":"alice","password":"pw"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("篡改 redirect 不得带 Location, got %q", loc)
	}
	if !strings.Contains(w.Body.String(), `"error":"invalid_request"`) {
		t.Errorf("body = %s", w.Body.String())
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（redirect 校验先于凭据）", env.ua.calls)
	}
}

// 成功：302 回跳带 code/state，回跳的 code 可被 Exchange 消费，凭据恰校验一次。
func TestAuthorizeLogin_OK_302WithCode(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://localhost:5173/callback","state":"xyz","scope":"all","username":"alice","password":"pw"}`)
	if w.Code != http.StatusFound {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "http://localhost:5173/callback?") {
		t.Fatalf("Location = %q", loc)
	}
	for _, want := range []string{"code=", "state=xyz"} {
		if !strings.Contains(loc, want) {
			t.Errorf("Location 缺 %q: %q", want, loc)
		}
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	code := u.Query().Get("code")
	if code == "" {
		t.Fatal("回跳缺 code")
	}
	if _, err := env.codes.Exchange(context.Background(), oauth2svc.ExchangeParams{
		Code: code, ClientID: "spa", RedirectURI: "http://localhost:5173/callback",
	}); err != nil {
		t.Errorf("回跳 code 应可消费: %v", err)
	}
	if len(env.ua.calls) != 1 {
		t.Errorf("Authenticate calls = %v, want 恰一次", env.ua.calls)
	}
}

// Review Focus #2：开关开 → 缺 captcha 字段 400 captcha_invalid，凭据不触达（authorize/login 路径）。
func TestAuthorizeLogin_Captcha_400(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.LoginPage = "/auth/login"
	cfg.Auth.Captcha.Enabled = true
	r, env := newFullRouter(t, cfg)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/authorize/login",
		`{"client_id":"spa","redirect_uri":"http://localhost:5173/callback","username":"alice","password":"pw"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "captcha_invalid") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Errorf("失败不得带 Location, got %q", loc)
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（captcha 先于凭据）", env.ua.calls)
	}
}

func TestToken_Password_OK_200(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"spa","username":"alice","password":"pw"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken != "at-1" || resp.RefreshToken != "rt-1" || resp.ExpiresIn != 7200 || resp.TokenType != "Bearer" {
		t.Errorf("resp = %+v", resp)
	}
	if len(env.ua.calls) != 1 {
		t.Errorf("Authenticate calls = %v, want 恰一次", env.ua.calls)
	}
}

// Review Focus #2：开关开 + 错误验证码 → 400 captcha_invalid（/token 路径），凭据不触达。
func TestToken_CaptchaInvalid_400(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.Captcha.Enabled = true
	r, env := newFullRouter(t, cfg)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"spa","username":"alice","password":"pw","captcha_uuid":"u1","captcha_code":"0000-wrong"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "captcha_invalid") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if len(env.ua.calls) != 0 {
		t.Errorf("Authenticate calls = %v, want 0（captcha 先于凭据）", env.ua.calls)
	}
}

func TestToken_UnknownGrant_400(t *testing.T) {
	r, env := newFullRouter(t, nil)
	seedSpaClient(t, env.clients)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","client_id":"spa"}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "unsupported_grant_type") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

func TestToken_ClientAuthFail_401(t *testing.T) {
	r, _ := newFullRouter(t, nil)
	w := postJSON(r, "/api/oauth2/token",
		`{"grant_type":"password","client_id":"nope"}`)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "invalid_client") {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}
