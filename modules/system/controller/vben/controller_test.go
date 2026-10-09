package vben

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

func newTestRouter(auth *fakeAuth, audit *fakeAudit, perm *fakePerm, user *fakeUser) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Register 挂在 /api 根组上（全路径 /api/auth/login），与 server 装配一致。
	NewController(auth, audit, perm, user).Register(r.Group("/api"))
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLogin_Success_Envelope(t *testing.T) {
	auth := &fakeAuth{loginRes: &service.LoginResult{
		AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 7200,
	}}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"admin123"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int     `json:"code"`
		Error *string `json:"error"`
		Data  struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
			ExpiresIn    int64  `json:"expiresIn"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 0 || resp.Error != nil {
		t.Errorf("envelope = %+v, want code=0 error=null", resp)
	}
	if resp.Data.AccessToken != "at-1" || resp.Data.RefreshToken != "rt-1" || resp.Data.ExpiresIn != 7200 {
		t.Errorf("data = %+v, want vben accessToken/refreshToken/expiresIn", resp.Data)
	}
	if len(audit.successes) != 1 || !audit.successes[0] {
		t.Errorf("audit successes = %v, want [true]", audit.successes)
	}
}

func TestLogin_WrongPassword_401Envelope(t *testing.T) {
	auth := &fakeAuth{loginErr: errInvalidCredentials}
	audit := &fakeAudit{}
	w := postJSON(newTestRouter(auth, audit, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `{"username":"admin","password":"nope"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code  int    `json:"code"`
		Data  any    `json:"data"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 401 || resp.Error == "" || resp.Data != nil {
		t.Errorf("envelope = %+v, want {code:401, data:null, error:非空}", resp)
	}
	if len(audit.successes) != 1 || audit.successes[0] {
		t.Errorf("audit successes = %v, want [false]", audit.successes)
	}
}

func TestLogin_BadBody_400(t *testing.T) {
	w := postJSON(newTestRouter(&fakeAuth{}, &fakeAudit{}, &fakePerm{}, &fakeUser{}),
		"/api/auth/login", `not-json`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}
