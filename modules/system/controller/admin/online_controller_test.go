package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/token"
)

// fakeOnlineSvc 控制桩：按用例切返回值。
type fakeOnlineSvc struct {
	list     []*service.OnlineSessionVO
	listErr  error
	total    int64
	kickErr  error
	kickedID uint64
	imp      *token.Issued
	impErr   error
	impUID   uint64
}

func (f *fakeOnlineSvc) List(ctx context.Context, page, size int) ([]*service.OnlineSessionVO, int64, error) {
	return f.list, f.total, f.listErr
}
func (f *fakeOnlineSvc) Kick(ctx context.Context, operator *auth.Claims, sessionID uint64) error {
	f.kickedID = sessionID
	return f.kickErr
}
func (f *fakeOnlineSvc) Impersonate(ctx context.Context, operator *auth.Claims, ip, userAgent string, targetUserID uint64) (*token.Issued, error) {
	f.impUID = targetUserID
	return f.imp, f.impErr
}

func newOnlineRouter(svc service.OnlineService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// 注入 claims 的桩中间件（与 pkg/middleware/auth.go 的 c.Set("claims", ...) 同键）
	r.Use(func(c *gin.Context) {
		c.Set("claims", &auth.Claims{UserID: 9, Username: "root", TenantID: 1, UserType: 1})
	})
	ctrl := NewOnlineController(svc)
	// 与 module.go RegisterRoutes 同形作回归
	r.GET("/api/admin/system/online", ctrl.List)
	r.DELETE("/api/admin/system/online/:id", ctrl.Kick)
	r.POST("/api/admin/system/online/impersonate", ctrl.Impersonate)
	return r
}

func doOnlineReq(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	var rd *strings.Reader = strings.NewReader(body)
	req := httptest.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func TestOnlineController_List(t *testing.T) {
	fake := &fakeOnlineSvc{
		list:  []*service.OnlineSessionVO{{ID: 1, UserID: 9, Username: "alice", ClientID: "vben-web"}},
		total: 1,
	}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodGet, "/api/admin/system/online?page=1&size=10", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 {
		t.Errorf("code = %d, msg %s", env.Code, env.Msg)
	}
	var data struct {
		List  []service.OnlineSessionVO `json:"list"`
		Total int64                     `json:"total"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.List) != 1 || data.Total != 1 || data.List[0].Username != "alice" {
		t.Errorf("data = %+v", data)
	}
	if strings.Contains(w.Body.String(), "access_token") {
		t.Error("response must not contain access_token field name from VO (明文防线)")
	}
}

func TestOnlineController_Kick(t *testing.T) {
	fake := &fakeOnlineSvc{}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/7", "")
	if w.Code != http.StatusOK || fake.kickedID != 7 {
		t.Errorf("status = %d, kickedID = %d", w.Code, fake.kickedID)
	}
	// 30202 透传
	fake.kickErr = errs.New(errs.CodeCannotKickSelf, nil)
	w = doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/7", "")
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 30202 {
		t.Errorf("code = %d, want 30202", env.Code)
	}
}

func TestOnlineController_Kick_BadID(t *testing.T) {
	fake := &fakeOnlineSvc{}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodDelete, "/api/admin/system/online/abc", "")
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != int(errs.CodeParamInvalid) {
		t.Errorf("code = %d, want 10001", env.Code)
	}
}

func TestOnlineController_Impersonate(t *testing.T) {
	fake := &fakeOnlineSvc{imp: &token.Issued{AccessToken: "a", RefreshToken: "r", ExpiresIn: 7200}}
	w := doOnlineReq(newOnlineRouter(fake), http.MethodPost, "/api/admin/system/online/impersonate", `{"user_id":501}`)
	if w.Code != http.StatusOK || fake.impUID != 501 {
		t.Fatalf("status = %d, impUID = %d, body %s", w.Code, fake.impUID, w.Body.String())
	}
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	var data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	_ = json.Unmarshal(env.Data, &data)
	if data.AccessToken != "a" || data.ExpiresIn != 7200 {
		t.Errorf("data = %+v", data)
	}
	// 30201 透传
	fake.impErr = errs.New(errs.CodeImpersonateMemberNotFound, nil)
	fake.imp = nil
	w = doOnlineReq(newOnlineRouter(fake), http.MethodPost, "/api/admin/system/online/impersonate", `{"user_id":502}`)
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 30201 {
		t.Errorf("code = %d, want 30201", env.Code)
	}
}
