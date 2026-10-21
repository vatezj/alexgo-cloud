package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/oauth2/social"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// === 夹具 ===

// newReq 构造带 JSON body 的请求（便于用例再补 Authorization 等头）。
func newReq(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serve(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doJSON 一次性发请求（无额外头的常规用例）。
func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	return serve(r, newReq(method, path, body))
}

// envelope 统一响应信封（resp.OK/Fail/Page 与503 unavailableBody 同构三键）。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var e envelope
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("unmarshal envelope: %v, body=%s", err, w.Body.String())
	}
	return e
}

// stubLoginBinder LoginBinder 测试桩：func 字段由各用例注入，
// 收到的参数落 got* 字段供断言（brief 夹具 + 记录字段）。
type stubLoginBinder struct {
	login      func(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error)
	bindSocial func(ctx context.Context, memberID uint64, platform, code string) error
	bindMobile func(ctx context.Context, memberID uint64, mobile string) error
	unbind     func(ctx context.Context, memberID uint64, platform string) error
	// 记录收到的参数
	gotPlatform string
	gotCode     string
	gotIP       string
	gotUA       string
	gotMemberID uint64
	gotMobile   string
	gotTenantID uint64
}

func (s *stubLoginBinder) Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error) {
	s.gotPlatform, s.gotCode, s.gotIP, s.gotUA = platform, code, ip, ua
	s.gotTenantID = tenant.TenantIDFromContext(ctx)
	return s.login(ctx, platform, code, ip, ua)
}

func (s *stubLoginBinder) BindSocial(ctx context.Context, memberID uint64, platform, code string) error {
	s.gotMemberID, s.gotPlatform, s.gotCode = memberID, platform, code
	s.gotTenantID = tenant.TenantIDFromContext(ctx)
	return s.bindSocial(ctx, memberID, platform, code)
}

func (s *stubLoginBinder) BindMobile(ctx context.Context, memberID uint64, mobile string) error {
	s.gotMemberID, s.gotMobile = memberID, mobile
	s.gotTenantID = tenant.TenantIDFromContext(ctx)
	return s.bindMobile(ctx, memberID, mobile)
}

func (s *stubLoginBinder) UnbindSocial(ctx context.Context, memberID uint64, platform string) error {
	s.gotMemberID, s.gotPlatform = memberID, platform
	s.gotTenantID = tenant.TenantIDFromContext(ctx)
	return s.unbind(ctx, memberID, platform)
}

// stubValidator token.Validator 测试桩（brief 夹具）。
type stubValidator struct {
	claims *token.Claims
	err    error
}

func (v *stubValidator) Validate(_ context.Context, _ string) (*token.Claims, error) {
	return v.claims, v.err
}

// newAppRouter 装一套 app 端路由（路径与 module.go RegisterRoutes 同款）。
func newAppRouter(t *testing.T, binder LoginBinder, validator token.Validator) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctrl := NewAppController(binder, validator)
	r := gin.New()
	r.POST("/api/app/social/wechat/mp/login", ctrl.LoginMP)
	r.POST("/api/app/social/wechat/web/login", ctrl.LoginWeb)
	r.POST("/api/app/member/auth/bind-social", ctrl.BindSocial)
	r.POST("/api/app/member/auth/bind-mobile", ctrl.BindMobile)
	r.DELETE("/api/app/member/auth/social/:platform", ctrl.UnbindSocial)
	return r
}

// memberClaims 构造 member 身份 claims。
func memberClaims(userID, tid uint64) *token.Claims {
	return &token.Claims{UserID: userID, Username: "u", UserType: token.UserTypeMember, TenantID: tid}
}

// === 登录（公开） ===

// TestAppLogin_MP_RoutesWechatMP POST mp/login {"code":"c"} →
// stub 收到 platform=wechat_mp，返回200 信封 code 0 + data 三元组。
func TestAppLogin_MP_RoutesWechatMP(t *testing.T) {
	stub := &stubLoginBinder{login: func(_ context.Context, _, _, _, _ string) (*oauth2svc.LoginResult, error) {
		return &oauth2svc.LoginResult{AccessToken: "tok-mp", RefreshToken: "ref-mp", ExpiresIn: 3600}, nil
	}}
	r := newAppRouter(t, stub, &stubValidator{})
	req := newReq(http.MethodPost, "/api/app/social/wechat/mp/login", `{"code":"c"}`)
	req.Header.Set("User-Agent", "mp-agent/1.0")
	w := serve(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	e := decode(t, w)
	if e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotPlatform != social.PlatformWechatMP {
		t.Errorf("platform=%q, want %q", stub.gotPlatform, social.PlatformWechatMP)
	}
	if stub.gotCode != "c" {
		t.Errorf("code=%q, want c", stub.gotCode)
	}
	if stub.gotUA != "mp-agent/1.0" {
		t.Errorf("ua=%q, want mp-agent/1.0（User-Agent 应透传）", stub.gotUA)
	}
	var data struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v, data=%s", err, e.Data)
	}
	if data.Token != "tok-mp" || data.RefreshToken != "ref-mp" || data.ExpiresIn != 3600 {
		t.Errorf("data = %+v, want token=tok-mp refresh_token=ref-mp expires_in=3600", data)
	}
}

// TestAppLogin_Web_RoutesWechatWeb 同 MP 用例，platform 须路由到 wechat_web。
func TestAppLogin_Web_RoutesWechatWeb(t *testing.T) {
	stub := &stubLoginBinder{login: func(_ context.Context, _, _, _, _ string) (*oauth2svc.LoginResult, error) {
		return &oauth2svc.LoginResult{AccessToken: "tok-web", RefreshToken: "ref-web", ExpiresIn: 1800}, nil
	}}
	r := newAppRouter(t, stub, &stubValidator{})
	w := doJSON(r, http.MethodPost, "/api/app/social/wechat/web/login", `{"code":"c2"}`)

	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	e := decode(t, w)
	if e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotPlatform != social.PlatformWechatWeb {
		t.Errorf("platform=%q, want %q", stub.gotPlatform, social.PlatformWechatWeb)
	}
	if stub.gotCode != "c2" {
		t.Errorf("code=%q, want c2", stub.gotCode)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(e.Data, &data); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if data.Token != "tok-web" {
		t.Errorf("data.token=%q, want tok-web", data.Token)
	}
}

// TestAppLogin_EmptyCode_10001 {} → 400 + CodeParamInvalid（10001），且不调 svc.Login。
func TestAppLogin_EmptyCode_10001(t *testing.T) {
	stub := &stubLoginBinder{login: func(_ context.Context, _, _, _, _ string) (*oauth2svc.LoginResult, error) {
		t.Error("code 空时不应调用 svc.Login")
		return nil, nil
	}}
	r := newAppRouter(t, stub, &stubValidator{})
	w := doJSON(r, http.MethodPost, "/api/app/social/wechat/mp/login", `{}`)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status=%d, want 400; body=%s", w.Code, w.Body.String())
	}
	if got := decode(t, w).Code; got != int(errs.CodeParamInvalid) {
		t.Errorf("code=%d, want %d; body=%s", got, errs.CodeParamInvalid, w.Body.String())
	}
}

// TestAppLogin_IssuanceErr_503 签发链路故障（ErrIssuance / ErrAuthenticatorMissing
// wrap）→ HTTP 503 + unavailableBody{code:50000,msg:"auth service unavailable",data:null}
// （与 member 登录同口径：I2 不冒充 401）。
func TestAppLogin_IssuanceErr_503(t *testing.T) {
	cases := map[string]error{
		"ErrIssuance":             fmt.Errorf("stub: issuer down: %w", oauth2svc.ErrIssuance),
		"ErrAuthenticatorMissing": fmt.Errorf("stub: %w", oauth2svc.ErrAuthenticatorMissing),
	}
	for name, sentinel := range cases {
		t.Run(name, func(t *testing.T) {
			stub := &stubLoginBinder{login: func(_ context.Context, _, _, _, _ string) (*oauth2svc.LoginResult, error) {
				return nil, sentinel
			}}
			r := newAppRouter(t, stub, &stubValidator{})
			w := doJSON(r, http.MethodPost, "/api/app/social/wechat/mp/login", `{"code":"c"}`)

			if w.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d, want 503; body=%s", w.Code, w.Body.String())
			}
			e := decode(t, w)
			if e.Code != int(errs.CodeInternal) || e.Msg != "auth service unavailable" || string(e.Data) != "null" {
				t.Errorf("envelope = {code:%d msg:%q data:%s}, want {50000 \"auth service unavailable\" null}; body=%s",
					e.Code, e.Msg, e.Data, w.Body.String())
			}
		})
	}
}

// === 绑定 / 解绑（Bearer 自鉴权） ===

// TestAppBindSocial_NoAuth_401 无 Authorization → 401 + CodeUnauthorized（20001），
// 且不触达 svc.BindSocial。
func TestAppBindSocial_NoAuth_401(t *testing.T) {
	stub := &stubLoginBinder{bindSocial: func(_ context.Context, _ uint64, _, _ string) error {
		t.Error("无 token 时不应调用 svc.BindSocial")
		return nil
	}}
	r := newAppRouter(t, stub, &stubValidator{err: errors.New("stub: no token")})
	w := doJSON(r, http.MethodPost, "/api/app/member/auth/bind-social", `{"platform":"wechat_mp","code":"c"}`)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status=%d, want 401; body=%s", w.Code, w.Body.String())
	}
	if got := decode(t, w).Code; got != int(errs.CodeUnauthorized) {
		t.Errorf("code=%d, want %d", got, errs.CodeUnauthorized)
	}
}

// TestAppBindSocial_WrongUserType_403 管理员 token（UserType≠member）→ 403 + 20003。
func TestAppBindSocial_WrongUserType_403(t *testing.T) {
	stub := &stubLoginBinder{bindSocial: func(_ context.Context, _ uint64, _, _ string) error {
		t.Error("非 member 身份不应调用 svc.BindSocial")
		return nil
	}}
	r := newAppRouter(t, stub, &stubValidator{claims: &token.Claims{
		UserID: 9, UserType: token.UserTypeAdmin, TenantID: 1,
	}})
	req := newReq(http.MethodPost, "/api/app/member/auth/bind-social", `{"platform":"wechat_mp","code":"c"}`)
	req.Header.Set("Authorization", "Bearer admin-tok")
	w := serve(r, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status=%d, want 403; body=%s", w.Code, w.Body.String())
	}
	if got := decode(t, w).Code; got != int(errs.CodeForbidden) {
		t.Errorf("code=%d, want %d", got, errs.CodeForbidden)
	}
}

// TestAppBindSocial_OK_PassesUserID Bearer + 合法 body → stub 收
// memberID=claims.UserID（且 claims 租户已写回请求 ctx，service 才能隔离）。
func TestAppBindSocial_OK_PassesUserID(t *testing.T) {
	stub := &stubLoginBinder{bindSocial: func(_ context.Context, _ uint64, _, _ string) error { return nil }}
	r := newAppRouter(t, stub, &stubValidator{claims: memberClaims(100, 7)})
	req := newReq(http.MethodPost, "/api/app/member/auth/bind-social", `{"platform":"wechat_mp","code":"c"}`)
	req.Header.Set("Authorization", "Bearer tok-100")
	w := serve(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotMemberID != 100 {
		t.Errorf("memberID=%d, want 100（claims.UserID）", stub.gotMemberID)
	}
	if stub.gotPlatform != social.PlatformWechatMP || stub.gotCode != "c" {
		t.Errorf("platform/code = %q/%q, want wechat_mp/c", stub.gotPlatform, stub.gotCode)
	}
	if stub.gotTenantID != 7 {
		t.Errorf("ctx tenant=%d, want 7（claims.TenantID 应写回请求 ctx）", stub.gotTenantID)
	}
}

// TestAppBindMobile_OK Bearer + {"mobile":...} → stub 收 memberID + mobile。
func TestAppBindMobile_OK(t *testing.T) {
	stub := &stubLoginBinder{bindMobile: func(_ context.Context, _ uint64, _ string) error { return nil }}
	r := newAppRouter(t, stub, &stubValidator{claims: memberClaims(100, 7)})
	req := newReq(http.MethodPost, "/api/app/member/auth/bind-mobile", `{"mobile":"13800138000"}`)
	req.Header.Set("Authorization", "Bearer tok-100")
	w := serve(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotMemberID != 100 || stub.gotMobile != "13800138000" {
		t.Errorf("memberID/mobile = %d/%q, want 100/13800138000", stub.gotMemberID, stub.gotMobile)
	}
}

// TestAppUnbind_PlatformParam DELETE /app/member/auth/social/wechat_mp →
// 平台取自路径参数，memberID 取自 claims。
func TestAppUnbind_PlatformParam(t *testing.T) {
	stub := &stubLoginBinder{unbind: func(_ context.Context, _ uint64, _ string) error { return nil }}
	r := newAppRouter(t, stub, &stubValidator{claims: memberClaims(100, 7)})
	req := newReq(http.MethodDelete, "/api/app/member/auth/social/wechat_mp", "")
	req.Header.Set("Authorization", "Bearer tok-100")
	w := serve(r, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if e := decode(t, w); e.Code != 0 {
		t.Errorf("envelope code=%d, want 0; body=%s", e.Code, w.Body.String())
	}
	if stub.gotPlatform != social.PlatformWechatMP {
		t.Errorf("platform=%q, want wechat_mp（路径参数）", stub.gotPlatform)
	}
	if stub.gotMemberID != 100 {
		t.Errorf("memberID=%d, want 100", stub.gotMemberID)
	}
}
