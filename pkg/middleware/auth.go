package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// AuthDeps 聚合鉴权中间件的可选依赖。
//
// 双模式（cfg.Auth.Mode）：
// - token（默认）：不透明令牌经 Validator 查库校验，claims 含 user_type/dept_id；
// - jwt：兼容旧路径（静态密钥 JWT），作为回滚开关。
// Enforcer/Validator 均可为 nil：nil Enforcer 跳过 Casbin；mode=token 但 Validator 为 nil
// 属装配错误，直接 401 拒绝（fail-closed，绝不放行）。
type AuthDeps struct {
	Cfg       *config.Config
	Enforcer  *casbin.Enforcer
	Validator token.Validator
}

func NewAuthMiddleware(d AuthDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/health" || path == "/health/ready" || path == "/metrics" ||
			strings.HasPrefix(path, "/debug/pprof/") {
			c.Next()
			return
		}
		// 公开路径：非 /api/admin/** 且非需登录的 app 接口一律放行。
		// /api/app/** 中仅本清单需要登录态（登录/注册等入口是公开的）。
		if !strings.HasPrefix(path, "/api/admin/") && !appAuthRequired(path) {
			c.Next()
			return
		}

		tokenStr := c.GetHeader("Authorization")
		claims, err := parseClaims(d, tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("claims", claims)

		if d.Enforcer != nil {
			// sub 带租户前缀：{tenantId}:{username}，杜绝跨租户同名角色串策略（块⑦）。
			sub := fmt.Sprintf("%d:%s", claims.TenantID, claims.Username)
			if claims.Username == "" {
				sub = fmt.Sprintf("%d:%d", claims.TenantID, claims.UserID)
			}
			obj := c.FullPath()
			if obj == "" {
				obj = path
			}
			ok, eerr := d.Enforcer.Enforce(sub, obj, c.Request.Method)
			if eerr != nil || !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}
		c.Next()
	}
}

// appAuthRequired：app 端需要登录态的路径前缀（一期只有 member 的登出/刷新；
// 登录、注册、send-sms-code 等入口公开）。
func appAuthRequired(path string) bool {
	return strings.HasPrefix(path, "/api/app/member/auth/logout") ||
		strings.HasPrefix(path, "/api/app/member/auth/refresh")
}

func parseClaims(d AuthDeps, tokenStr string) (*auth.Claims, error) {
	if d.Cfg != nil && d.Cfg.Auth.Mode == "jwt" {
		return auth.ParseToken(tokenStr, d.Cfg) // 旧路径：Bearer JWT
	}
	// token 模式（默认）
	if d.Validator == nil {
		return nil, fmt.Errorf("auth: validator not wired (mode=token)")
	}
	if tokenStr == "" {
		return nil, fmt.Errorf("auth: empty token")
	}
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenStr), "Bearer "))
	tc, err := d.Validator.Validate(context.Background(), raw)
	if err != nil {
		return nil, err
	}
	return &auth.Claims{
		UserID:   tc.UserID,
		Username: tc.Username,
		TenantID: tc.TenantID,
		UserType: int(tc.UserType),
		DeptID:   tc.DeptID,
	}, nil
}
