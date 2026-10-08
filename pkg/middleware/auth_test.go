package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/middleware"
)

func testCfg() *config.Config {
	c := &config.Config{}
	c.System.JWTSecret = "middleware-test-secret"
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
	r.Use(middleware.AuthMiddleware(cfg, enf))
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
	if _, err := enf.AddPolicy("bob", "/api/admin/system/users", "GET"); err != nil {
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
	if _, err := enf.AddPolicy("alice", "/api/admin/system/users", "GET"); err != nil {
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
