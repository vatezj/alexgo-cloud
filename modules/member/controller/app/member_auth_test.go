package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
)

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
func TestLogin_NilIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: issuer not wired", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"error":"auth service unavailable"}` {
		t.Errorf("body = %s", w.Body.String())
	}
}

// I2：issuer 故障（wrap ErrIssuance）→ 503。
func TestLogin_BadIssuer_503(t *testing.T) {
	u := &stubUnified{err: fmt.Errorf("%w: deadline exceeded", oauth2svc.ErrIssuance)}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", w.Code, w.Body.String())
	}
}

// 凭据错误 → 401（与 503 判别：签发故障绝不冒充密码错误，反之亦然）。
func TestLogin_BadPassword_401(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrInvalidCredentials}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"wrong-pass"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	// 哨兵文案与旧字符串逐字相同 → body 不变（裁决 3）。
	if w.Body.String() != `{"error":"invalid credentials"}` {
		t.Errorf("body = %s", w.Body.String())
	}
}

// 正常签发 → 200 且返回 token 三元组；登录参数按裁决 2 固定。
func TestLogin_OK_200(t *testing.T) {
	u := &stubUnified{res: &oauth2svc.LoginResult{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"token":"a"`) {
		t.Errorf("body = %s, want token payload", w.Body.String())
	}
	if len(u.logins) != 1 || u.logins[0].AccountType != oauth2svc.AccountTypeMember ||
		u.logins[0].ClientID != "alexgo-app" || u.logins[0].Username != "13800000000" {
		t.Errorf("logins = %+v", u.logins)
	}
}

// Review Focus #5：三端 captcha 映射——member 端 400。
func TestLogin_CaptchaInvalid_400(t *testing.T) {
	u := &stubUnified{err: oauth2svc.ErrCaptchaInvalid}
	w := doLogin(t, u, `{"mobile":"13800000000","password":"pw123456","captcha_uuid":"u","captcha_code":"y"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}
