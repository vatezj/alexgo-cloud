package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/token"
)

// stubMeSvc MemberService 测试桩：只覆盖 Me/UpdateMe；嵌入 nil 接口让其余方法
// 调用即 panic（本文件用例不应触达）。
type stubMeSvc struct {
	service.MemberService
	me      *service.ProfileResult
	meErr   error
	upd     *service.ProfileResult
	updErr  error
	updNick *string
	updAva  *string
	updUID  uint64
}

func (s *stubMeSvc) Me(_ context.Context, userID uint64) (*service.ProfileResult, error) {
	if s.meErr != nil {
		return nil, s.meErr
	}
	return s.me, nil
}

func (s *stubMeSvc) UpdateMe(_ context.Context, userID uint64, nickname, avatar *string) (*service.ProfileResult, error) {
	s.updUID, s.updNick, s.updAva = userID, nickname, avatar
	if s.updErr != nil {
		return nil, s.updErr
	}
	return s.upd, nil
}

// stubValidator token.Validator 测试桩（与 scorecard app_test 同形，本包独立命名空间）。
type stubValidator struct{ byToken map[string]*token.Claims }

func (s stubValidator) Validate(_ context.Context, accessToken string) (*token.Claims, error) {
	if cl, ok := s.byToken[accessToken]; ok {
		return cl, nil
	}
	return nil, context.Canceled // 非法 token → claims 失败
}

var meProfile = &service.ProfileResult{
	UserID: 7, Nickname: "tester", Avatar: "a1",
	CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
}

// doMe 走 httptest 完整路由（复刻 module 注册的 GET/PUT /api/app/member/me，
// Bearer 自鉴权模式与 scorecard app.go claims 同构）。
func doMe(t *testing.T, svc *stubMeSvc, method, path, auth, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	c := NewMeController(svc, stubValidator{byToken: map[string]*token.Claims{
		"tok": {UserID: 7, Username: "tester", UserType: token.UserTypeMember, TenantID: 1},
	}})
	r.GET("/api/app/member/me", c.Me)
	r.PUT("/api/app/member/me", c.UpdateMe)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// GET me：Bearer 鉴权后 200，data 的 json 键名逐字 user_id/nickname/avatar/created_at。
func TestMe_OK(t *testing.T) {
	w := doMe(t, &stubMeSvc{me: meProfile}, http.MethodGet, "/api/app/member/me", "tok", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if code, msg, _ := codeOf(t, w); code != 0 || msg != "ok" {
		t.Errorf("code/msg = %d/%q, want 0/ok; body=%s", code, msg, w.Body.String())
	}
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, w.Body.String())
	}
	for _, key := range []string{"user_id", "nickname", "avatar", "created_at"} {
		if _, ok := body.Data[key]; !ok {
			t.Errorf("data 缺键 %q: %s", key, w.Body.String())
		}
	}
	// 泄漏面：mobile/password 等敏感列不得出现在 me 响应里。
	for _, leak := range []string{"mobile", "password", "tenant_id", "status"} {
		if _, ok := body.Data[leak]; ok {
			t.Errorf("data 泄漏敏感键 %q: %s", leak, w.Body.String())
		}
	}
	if v := string(body.Data["user_id"]); v != "7" {
		t.Errorf("user_id = %s, want 7", v)
	}
	if v := string(body.Data["nickname"]); v != `"tester"` {
		t.Errorf("nickname = %s, want \"tester\"", v)
	}
}

// PUT me：只传 nickname（部分字段）→ 200 且透传指针给 service（avatar 位 nil）。
func TestMe_Update_OK(t *testing.T) {
	svc := &stubMeSvc{upd: &service.ProfileResult{
		UserID: 7, Nickname: "新昵称", Avatar: "a1",
		CreatedAt: meProfile.CreatedAt,
	}}
	w := doMe(t, svc, http.MethodPut, "/api/app/member/me", "tok", `{"nickname":"新昵称"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if code, _, _ := codeOf(t, w); code != 0 {
		t.Errorf("code = %d, want 0; body=%s", code, w.Body.String())
	}
	if svc.updUID != 7 || svc.updNick == nil || *svc.updNick != "新昵称" || svc.updAva != nil {
		t.Errorf("UpdateMe(uid,nick,ava) = %d/%v/%v, want 7/\"新昵称\"/nil", svc.updUID, svc.updNick, svc.updAva)
	}
	if v := string(mustData(t, w)["nickname"]); v != `"新昵称"` {
		t.Errorf("resp nickname = %s", v)
	}
}

// PUT me：nickname 空串 → 10001（service 拒绝原样透传）。
func TestMe_Update_EmptyNickname_10001(t *testing.T) {
	svc := &stubMeSvc{updErr: errs.New(errs.CodeParamInvalid, nil)}
	w := doMe(t, svc, http.MethodPut, "/api/app/member/me", "tok", `{"nickname":""}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	code, msg, data := codeOf(t, w)
	if code != 10001 || msg != "参数校验失败" || data != nil {
		t.Errorf("code/msg/data = %d/%q/%v, want 10001/参数校验失败/<nil>; body=%s",
			code, msg, data, w.Body.String())
	}
}

// mustData 解析信封 data 为 map（断言专用，失败即 Fatalf）。
func mustData(t *testing.T, w *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var body struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v, body=%s", err, w.Body.String())
	}
	return body.Data
}
