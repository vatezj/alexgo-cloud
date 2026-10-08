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
// 便于 Casbin sub 前缀 {tenantId}:{username} 在两种模式下一致）。
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
	// sub 前缀 {tenantId}:{username}（Task 3 块⑦）：策略 subject 与中间件 sub 同构。
	if _, err := enf.AddPolicy("1:bob", "/api/admin/system/users", "GET"); err != nil {
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
	// sub 前缀 {tenantId}:{username}（Task 3 块⑦）：策略 subject 与中间件 sub 同构。
	if _, err := enf.AddPolicy("1:alice", "/api/admin/system/users", "GET"); err != nil {
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
