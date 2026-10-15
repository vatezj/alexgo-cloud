package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
)

// stubUnified 本包独立夹具（system/controller/app 与 vben 是不同包，不能共用）。
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

func appLogin(t *testing.T, u *stubUnified, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/app/system/auth/login", NewAppController(u).Login)
	req := httptest.NewRequest(http.MethodPost, "/api/app/system/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAppLogin_Quirks 冻结 app 端怪癖契约（Review Focus #5 三端兼容之一）：
// 成功 200 三元组；凭据错 500（不是 401，怪癖）；captcha 拒绝 400；body 非 JSON 400。
func TestAppLogin_Quirks(t *testing.T) {
	t.Run("success 200 triple", func(t *testing.T) {
		u := &stubUnified{res: &oauth2svc.LoginResult{AccessToken: "at", RefreshToken: "rt", ExpiresIn: 60}}
		w := appLogin(t, u, `{"username":"a","password":"b"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		for _, want := range []string{`"token":"at"`, `"refresh_token":"rt"`, `"expires_in":60`} {
			if !strings.Contains(w.Body.String(), want) {
				t.Errorf("body 缺 %s: %s", want, w.Body.String())
			}
		}
		if len(u.logins) != 1 || u.logins[0].ClientID != "alexgo-admin" ||
			u.logins[0].AccountType != oauth2svc.AccountTypeSystem {
			t.Errorf("logins = %+v", u.logins)
		}
	})
	t.Run("bad credentials 500 quirk", func(t *testing.T) {
		u := &stubUnified{err: oauth2svc.ErrInvalidCredentials}
		w := appLogin(t, u, `{"username":"a","password":"b"}`)
		if w.Code != http.StatusInternalServerError ||
			!strings.Contains(w.Body.String(), "invalid credentials") {
			t.Fatalf("code=%d body=%s（怪癖：凭据错也 500）", w.Code, w.Body.String())
		}
	})
	t.Run("captcha rejected 400", func(t *testing.T) {
		u := &stubUnified{err: oauth2svc.ErrCaptchaInvalid}
		w := appLogin(t, u, `{"username":"a","password":"b","captcha_uuid":"u","captcha_code":"y"}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})
	t.Run("bad body 400", func(t *testing.T) {
		w := appLogin(t, &stubUnified{}, `not-json`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})
}
