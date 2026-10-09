package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// fakeValidator：token.Validator 的测试替身。
// jwt 模式下中间件不调用它（仅保证依赖非 nil）；token 模式下按注入值返回 claims/错误。
type fakeValidator struct {
	claims *token.Claims
	err    error
}

func (f fakeValidator) Validate(_ context.Context, _ string) (*token.Claims, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.claims, nil
}

// claimsFor 生成与 jwt 用例同源的默认 claims（TenantID=1 与 GenerateToken 的 tenantID 参数对齐，
// 便于 Casbin sub 前缀 {tenantId}:{userType}:{username} 在两种模式下一致）。
func claimsFor(_ *config.Config) *token.Claims {
	return &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1,
	}
}

func testCfg() *config.Config {
	c := &config.Config{}
	c.System.JWTSecret = "middleware-test-secret"
	// 现有用例统一走 jwt 回归路径（旧 auth.ParseToken）；token 模式见 TestAuthMiddleware_TokenMode。
	c.Auth.Mode = "jwt"
	return c
}

// newEnforcer 构建与 pkg/auth/casbin.go 同款 model 的内存 Casbin。
func newEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(`
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)
`)
	if err != nil {
		t.Fatal(err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func newRouter(cfg *config.Config, enf *casbin.Enforcer) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: fakeValidator{claims: claimsFor(cfg)}}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		claimsAny, _ := c.Get("claims")
		claims, _ := claimsAny.(*auth.Claims)
		name := ""
		if claims != nil {
			name = claims.Username
		}
		c.JSON(http.StatusOK, gin.H{"username": name})
	})
	r.POST("/api/app/system/auth/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func do(r *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthMiddleware_AdminWithoutToken_401(t *testing.T) {
	w := do(newRouter(testCfg(), nil), "GET", "/api/admin/system/users", "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_AdminBadToken_401(t *testing.T) {
	w := do(newRouter(testCfg(), nil), "GET", "/api/admin/system/users", "Bearer not-a-jwt")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMiddleware_AdminValidToken_200InjectsClaims(t *testing.T) {
	cfg := testCfg()
	token, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, nil), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"username":"alice"}` {
		t.Errorf("body = %s, want claims injected into context", body)
	}
}

func TestAuthMiddleware_NonAdminPaths_WithoutToken_200(t *testing.T) {
	r := newRouter(testCfg(), nil)
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/app/system/auth/login"},
		{"GET", "/health"},
	} {
		if w := do(r, tc.method, tc.path, ""); w.Code != http.StatusOK {
			t.Errorf("%s %s: status = %d, want 200 (public path)", tc.method, tc.path, w.Code)
		}
	}
}

func TestAuthMiddleware_CasbinDeny_403(t *testing.T) {
	cfg := testCfg()
	enf := newEnforcer(t)
	// 只给 bob 授权；alice 请求 → 403。
	// sub 前缀 {tenantId}:{userType}:{username}（C1）：策略 subject 与中间件 sub 同构。
	if _, err := enf.AddPolicy("1:1:bob", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	token, err := auth.GenerateToken(1, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, enf), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestAuthMiddleware_CasbinAllow_200(t *testing.T) {
	cfg := testCfg()
	enf := newEnforcer(t)
	// sub 前缀 {tenantId}:{userType}:{username}（C1）：策略 subject 与中间件 sub 同构。
	if _, err := enf.AddPolicy("1:1:alice", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	token, err := auth.GenerateToken(1, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	w := do(newRouter(cfg, enf), "GET", "/api/admin/system/users", "Bearer "+token)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// jwt 模式：旧静态 JWT 的 claims 恒为管理员类型（Task 10 数据权限加载器依赖 UserType==1）。
func TestAuthMiddleware_JwtMode_UserTypeAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg() // Auth.Mode = "jwt"
	legacy, err := auth.GenerateToken(9, "alice", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: fakeValidator{claims: &token.Claims{UserType: token.UserTypeMember}}}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		a, _ := c.Get("claims")
		c.JSON(200, gin.H{"ut": a.(*auth.Claims).UserType})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer "+legacy)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	// Validator 被忽略（jwt 模式）且 UserType 被盖成 1，而非 validator 的 2。
	if w.Body.String() != `{"ut":1}` {
		t.Errorf("body = %s, want {\"ut\":1}", w.Body.String())
	}
}

// token 模式：合法 validator → 200 且 claims 注入（含 user_type/dept_id）；
// validator 报错 → 401。
func TestAuthMiddleware_TokenMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"

	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		a, _ := c.Get("claims")
		cl := a.(*auth.Claims)
		c.JSON(200, gin.H{"ut": cl.UserType, "dept": cl.DeptID})
	})

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != `{"dept":77,"ut":1}` {
		t.Errorf("body = %s", w.Body.String())
	}

	// validator 失败 → 401
	r2 := gin.New()
	r2.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg: cfg, Validator: fakeValidator{err: errors.New("bad")},
	}))
	r2.GET("/api/admin/system/users", func(c *gin.Context) { c.Status(200) })
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, httptest.NewRequest("GET", "/api/admin/system/users", nil))
	if w2.Code != 401 {
		t.Errorf("status = %d, want 401", w2.Code)
	}

	// 装配错误 fail-closed：mode=token 但 Validator 未注入 → 401，绝不放行。
	r3 := gin.New()
	r3.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg}))
	r3.GET("/api/admin/system/users", func(c *gin.Context) { c.Status(200) })
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req3.Header.Set("Authorization", "Bearer whatever")
	r3.ServeHTTP(w3, req3)
	if w3.Code != 401 {
		t.Errorf("no-validator status = %d, want 401 (fail-closed)", w3.Code)
	}
}

// T3 移交①：token 模式下 Casbin sub 同样带 {tenantId}:{userType}:{username} 前缀。
// 策略 subject 为 "1:1:alice"；若中间件不加前缀（sub="alice"）则不命中 → 403，
// 因此 200 判别前缀已参与 Enforce；再用跨租户 claims 断言前缀确实隔离。
func TestAuthMiddleware_TokenMode_CasbinPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	enf := newEnforcer(t)
	if _, err := enf.AddPolicy("1:1:alice", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	build := func(tid uint64) *gin.Engine {
		v := fakeValidator{claims: &token.Claims{
			UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: tid,
		}}
		r := gin.New()
		r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: v}))
		r.GET("/api/admin/system/users", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		return r
	}
	doAuth := func(r *gin.Engine) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
		req.Header.Set("Authorization", "Bearer t")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 同租户：sub "1:1:alice" 命中带前缀策略 → 200。
	if w := doAuth(build(1)); w.Code != http.StatusOK {
		t.Errorf("token-mode prefixed sub status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	// 跨租户：sub "2:1:alice" 不命中 "1:1:alice" → 403（判别：前缀参与 Enforce 且隔离）。
	if w := doAuth(build(2)); w.Code != http.StatusForbidden {
		t.Errorf("cross-tenant sub status = %d, want 403", w.Code)
	}
}

// T3 移交②：claims.Username 为空 → sub 退化为 {tid}:{ut}:{userid}，仍能命中以 userid 为
// sub 的 g 绑定（g("1:1:9","1:role:editor") → p("1:role:editor",…)）。若未退化则 sub="1:1:" 不命中。
func TestAuthMiddleware_EmptyUsername_SubUserIDFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	enf := newEnforcer(t)
	if _, err := enf.AddGroupingPolicy("1:1:9", "1:role:editor"); err != nil {
		t.Fatal(err)
	}
	if _, err := enf.AddPolicy("1:role:editor", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "", UserType: token.UserTypeAdmin, TenantID: 1,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("empty-username sub {tid}:{ut}:{userid} status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// ---- Task 10：数据权限 ScopeLoader 注入 ----

// fakeScope：tenant.ScopeLoader 测试替身（记录调用与入参）。
type fakeScope struct {
	ds       tenant.DataScope
	err      error
	calls    int
	lastUser uint64
	lastDept uint64
}

func (f *fakeScope) Load(_ context.Context, userID, deptID uint64) (tenant.DataScope, error) {
	f.calls++
	f.lastUser, f.lastDept = userID, deptID
	if f.err != nil {
		return tenant.DataScope{}, f.err
	}
	return f.ds, nil
}

// scopeRouter：token 模式 + 指定 claims/加载器；handler 回报 DataScope 是否注入及档位。
func scopeRouter(cfg *config.Config, claims *token.Claims, sl tenant.ScopeLoader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{
		Cfg: cfg, Validator: fakeValidator{claims: claims}, ScopeLoader: sl,
	}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		ds, ok := tenant.DataContext(c.Request.Context())
		c.JSON(http.StatusOK, gin.H{"ok": ok, "mode": ds.Mode, "user": ds.UserID, "dept": ds.DeptID})
	})
	return r
}

func scopeReq(r *gin.Engine) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 管理员：加载器结果注入请求 ctx（入参为 claims 的 userID/deptID）。
func TestAuthMiddleware_ScopeInjected_ForAdmin(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	loader := &fakeScope{ds: tenant.DataScope{Mode: 3, UserID: 9, DeptID: 77}}
	claims := &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}
	w := scopeReq(scopeRouter(cfg, claims, loader))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"dept":77,"mode":3,"ok":true,"user":9}` {
		t.Errorf("body = %s, want scope injected (mode 3)", w.Body.String())
	}
	if loader.calls != 1 || loader.lastUser != 9 || loader.lastDept != 77 {
		t.Errorf("loader calls=%d user=%d dept=%d, want 1 call with (9,77)", loader.calls, loader.lastUser, loader.lastDept)
	}
}

// 加载器失败 → 注入 Mode 5（仅本人，最严方向），请求不被阻断。
func TestAuthMiddleware_ScopeLoadError_FallbackMode5(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	loader := &fakeScope{err: errors.New("db down")}
	claims := &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}
	w := scopeReq(scopeRouter(cfg, claims, loader))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"dept":77,"mode":5,"ok":true,"user":9}` {
		t.Errorf("body = %s, want fallback Mode 5", w.Body.String())
	}
}

// member token 打 /api/admin/**：C1 管理员门槛 403（早于 scope 注入与 Enforcer——
// 本路由未挂 enforcer，顺带钉住 I4：门槛不依赖 enforcer 装配），
// ScopeLoader 零调用（member 永不注入数据范围）。
func TestAuthMiddleware_ScopeSkipped_ForMember(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	loader := &fakeScope{ds: tenant.DataScope{Mode: 1}}
	claims := &token.Claims{
		UserID: 5, Username: "bob", UserType: token.UserTypeMember, TenantID: 1, DeptID: 77,
	}
	w := scopeReq(scopeRouter(cfg, claims, loader))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d want 403 (admin-only gate) body=%s", w.Code, w.Body.String())
	}
	if loader.calls != 0 {
		t.Errorf("loader called %d times for member, want 0", loader.calls)
	}
}

// ScopeLoader 为 nil（member 端/未启用装配）→ 不注入、不 panic。
func TestAuthMiddleware_ScopeLoaderNil(t *testing.T) {
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	claims := &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1, DeptID: 77,
	}
	w := scopeReq(scopeRouter(cfg, claims, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"dept":0,"mode":0,"ok":false,"user":0}` {
		t.Errorf("body = %s, want no scope when loader nil", w.Body.String())
	}
}

// ---- C1：管理员路径门槛（user_type 维度） ----

// member 昵称撞管理员用户名（sub 撞名场景）：即便 enforcer 里已有
// g("0:1:admin","0:role:admin") + 角色全量策略，member（ut=2）打 /api/admin/** 也必须 403——
// 门槛在 Enforcer 之前拦截，即使 sub 完全撞名也进不了策略判定（I4：micro member-server 无 enforcer 同样生效）。
func TestAuthMiddleware_MemberOnAdminPath_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	enf := newEnforcer(t)
	if _, err := enf.AddGroupingPolicy("0:1:admin", "0:role:admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := enf.AddPolicy("0:role:admin", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	v := fakeValidator{claims: &token.Claims{
		UserID: 5, Username: "admin", UserType: token.UserTypeMember, TenantID: 0,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("member on admin path status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"error":"forbidden"}` {
		t.Errorf("body = %s, want {\"error\":\"forbidden\"}", body)
	}
}

// 同一 enforcer、同一撞名用户名，管理员（ut=1）正常走 Enforcer → 200。
// 与上一用例合起来判别：403 来自 user_type 门槛，而非策略缺失。
func TestAuthMiddleware_AdminOnAdminPath_EnforcerAllows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	enf := newEnforcer(t)
	if _, err := enf.AddGroupingPolicy("0:1:admin", "0:role:admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := enf.AddPolicy("0:role:admin", "/api/admin/system/users", "GET"); err != nil {
		t.Fatal(err)
	}

	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "admin", UserType: token.UserTypeAdmin, TenantID: 0,
	}}
	r := gin.New()
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("admin on admin path status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// ---- C2：Token 内租户覆盖 X-Tenant-ID 头（spec §4.3） ----

// 复刻 http.go 顺序：TenantMiddleware 在 AuthMiddleware 之前。
// header X-Tenant-ID: 2 + token claims tenant 1 → handler 里 TenantID 必须为 1
//（判别：AuthMiddleware 未按 token 回写 ctx 则为 2 → 头伪造跨租户成功）。
func TestAuthMiddleware_TokenTenantOverridesHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"

	v := fakeValidator{claims: &token.Claims{
		UserID: 9, Username: "alice", UserType: token.UserTypeAdmin, TenantID: 1,
	}}
	r := gin.New()
	r.Use(middleware.NewTenantMiddleware(nil))
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Validator: v}))
	r.GET("/api/admin/system/users", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"tid": tenant.TenantIDFromContext(c.Request.Context())})
	})

	req := httptest.NewRequest("GET", "/api/admin/system/users", nil)
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("X-Tenant-ID", "2")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Body.String() != `{"tid":1}` {
		t.Errorf("body = %s, want tid=1（token 优先于 X-Tenant-ID 头）", w.Body.String())
	}
}

// ---- I1：member refresh/logout 走公开路径 ----

// 复刻 http.go 顺序（Tenant + Auth，带 enforcer 与 token 校验器）。
// member refresh 是 possession-based（凭 body 的 refresh_token）：access 过期/缺失都不该
// 被中间件拦下（controller 层的 400/401 属业务）。判别：中间件若仍鉴权 → 401/403。
func TestAuthMiddleware_MemberRefresh_NotIntercepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testCfg()
	cfg.Auth.Mode = "token"
	enf := newEnforcer(t)
	// 即便 enforcer 一条策略都没有，也不该影响公开路径。
	v := fakeValidator{err: errors.New("expired")} // 过期 access：根本轮不到校验

	handlerHit := false
	r := gin.New()
	r.Use(middleware.NewTenantMiddleware(nil))
	r.Use(middleware.NewAuthMiddleware(middleware.AuthDeps{Cfg: cfg, Enforcer: enf, Validator: v}))
	r.POST("/api/app/member/auth/refresh", func(c *gin.Context) {
		handlerHit = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.POST("/api/app/member/auth/logout", func(c *gin.Context) {
		handlerHit = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for _, path := range []string{"/api/app/member/auth/refresh", "/api/app/member/auth/logout"} {
		handlerHit = false
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("Authorization", "Bearer expired-access") // 带过期/无效 access token
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("POST %s status = %d, want 200（中间件放行）; body=%s", path, w.Code, w.Body.String())
		}
		if !handlerHit {
			t.Errorf("POST %s 未到达 handler（被中间件拦截）", path)
		}
	}
}
