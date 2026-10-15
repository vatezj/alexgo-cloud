package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// codeOf 解析统一信封的 code/msg/data（旧 {"error":...} 精确串断言迁移为信封三键断言）。
func codeOf(t *testing.T, w *httptest.ResponseRecorder) (int, string, any) {
	t.Helper()
	var body struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data any    `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal envelope: %v, body=%s", err, w.Body.String())
	}
	return body.Code, body.Msg, body.Data
}

// stubUnified 本包独立夹具（member/controller/app 独立命名空间）。
type stubUnified struct {
	res    *oauth2svc.LoginResult
	err    error
	logins []oauth2svc.LoginParams
}

func (s *stubUnified) Login(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.LoginResult, *oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.res, &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) AuthorizeLogin(_ context.Context, p oauth2svc.LoginParams) (*oauth2svc.AuthIdentity, error) {
	s.logins = append(s.logins, p)
	if s.err != nil {
		return nil, s.err
	}
	return &oauth2svc.AuthIdentity{UserID: 1, Username: p.Username, AccountType: p.AccountType}, nil
}
func (s *stubUnified) IssueFor(_ context.Context, _ *oauth2svc.AuthIdentity, _ string) (*oauth2svc.LoginResult, error) {
	return s.res, s.err
}

// doLogin 走 httptest 完整路由（复刻 module 注册的 POST /api/app/member/auth/login）。
// svc 归 Register/Refresh/Logout，Login 不再经 MemberService → 传 nil。
func doLogin(t *testing.T, unified *stubUnified, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewAuthController(nil, unified)
	r.POST("/api/app/member/auth/login", c.Login)
	req := httptest.NewRequest(http.MethodPost, "/api/app/member/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// I2：统一登录返回 wrap ErrIssuance → 503（签发故障绝不冒充 401）。
// 信封适配（不弱化）：HTTP 503 状态保持（I2 契约）；body 从旧 {"error":"auth service unavailable"}
// 变统一信封——code 必须精确 50000，文案与旧字面量逐字一致，data 为 null。
func TestLogin_NilIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: issuer not wired", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	code, msg, data := codeOf(t, w)
	if code != 50000 || msg != "auth service unavailable" || data != nil {
		t.Errorf("code/msg/data = %d/%q/%v, want 50000/auth service unavailable/<nil>; body=%s",
			code, msg, data, w.Body.String())
	}
}

// I2：issuer 故障（wrap ErrIssuance）→ 503（信封 code 50000）。
func TestLogin_BadIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: deadline exceeded", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	if code, _, _ := codeOf(t, w); code != 50000 {
		t.Errorf("code = %d, want 50000; body=%s", code, w.Body.String())
	}
}

// 凭据错误 → 401（与 503 判别：签发故障绝不冒充密码错误，反之亦然）。
// 信封适配（不弱化）：旧 body 精确串 {"error":"invalid credentials"} → code 精确 20001
// + 固定文案 + 原始错误串禁止外泄（Task 3 无泄漏规则）。
func TestLogin_BadPassword_401(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrInvalidCredentials}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"wrong-pass"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	code, msg, data := codeOf(t, w)
	if code != 20001 || msg != "未登录或登录已失效" || data != nil {
		t.Errorf("code/msg/data = %d/%q/%v, want 20001/未登录或登录已失效/<nil>; body=%s",
			code, msg, data, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "invalid credentials") {
		t.Errorf("原始错误泄漏到客户端: %s", w.Body.String())
	}
}

// 正常签发 → 200 且返回 token 三元组；登录参数按裁决 2 固定。
// 信封适配：token 三元组从顶层平铺移入 data（键名逐字不变），外加 code==0。
func TestLogin_OK_200(t *testing.T) {
	u := &stubUnified{res: &oauth2svc.LoginResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if code, msg, _ := codeOf(t, w); code != 0 || msg != "ok" {
		t.Errorf("code/msg = %d/%q, want 0/ok; body=%s", code, msg, w.Body.String())
	}
	for _, want := range []string{`"token":"a"`, `"refresh_token":"r"`, `"expires_in":7200`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("body 缺 %s: %s", want, w.Body.String())
		}
	}
	if len(u.logins) != 1 || u.logins[0].AccountType != oauth2svc.AccountTypeMember ||
		u.logins[0].ClientID != "alexgo-app" || u.logins[0].Username != "13800000000" {
		t.Errorf("logins = %+v", u.logins)
	}
}

// Review Focus #5：三端 captcha 映射——member 端 400（信封 code 精确 30005）。
func TestLogin_CaptchaInvalid_400(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrCaptchaInvalid}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456","captcha_uuid":"u","captcha_code":"y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if code, msg, _ := codeOf(t, w); code != 30005 || msg != "验证码错误" {
		t.Errorf("code/msg = %d/%q, want 30005/验证码错误; body=%s", code, msg, w.Body.String())
	}
}
