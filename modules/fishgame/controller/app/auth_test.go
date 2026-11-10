package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/repository"
	"alexGo-cloud/modules/fishgame/service"
)

func newAuthCtrl(t *testing.T) *AuthController {
	t.Helper()
	db := newAppTestDB(t)
	svc := service.NewAuthService(
		service.NewWechatClient("", ""),
		repository.NewPlayerRepository(db),
		repository.NewSessionRepository(db),
		[]string{"alice", "bob"}, "s3cret")
	return NewAuthController(svc)
}

// TestAuthCtrl_TestAccounts 名单拉取：200 {"list":[...]}，免 session。
func TestAuthCtrl_TestAccounts(t *testing.T) {
	ctrl := newAuthCtrl(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/auth/test_accounts", nil)
	ctrl.TestAccounts(c)
	if w.Code != http.StatusOK {
		t.Fatalf("应 200, got %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"alice"`) || !strings.Contains(w.Body.String(), `"bob"`) {
		t.Fatalf("list 应含 alice/bob: %s", w.Body.String())
	}
}

// TestAuthCtrl_TestLogin 名单内+对密钥 → 200 {"session":...}；错密钥/名单外 → 401 login_failed。
func TestAuthCtrl_TestLogin(t *testing.T) {
	ctrl := newAuthCtrl(t)
	gin.SetMode(gin.TestMode)

	post := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/auth/test_login", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		ctrl.TestLogin(c)
		return w
	}

	w := post(`{"account":"alice","secret":"s3cret"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"session"`) {
		t.Fatalf("成功应 200 {session}: %d %s", w.Code, w.Body.String())
	}
	w = post(`{"account":"alice","secret":"wrong"}`)
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "login_failed") {
		t.Fatalf("错密钥应 401 login_failed: %d %s", w.Code, w.Body.String())
	}
	w = post(`{"account":"mallory","secret":"s3cret"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("名单外应 401: %d %s", w.Code, w.Body.String())
	}
	w = post(`{}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("缺字段应 400: %d %s", w.Code, w.Body.String())
	}
}
