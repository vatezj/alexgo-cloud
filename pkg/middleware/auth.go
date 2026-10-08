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
	"alexGo-cloud/pkg/tenant"
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
	// ScopeLoader：数据权限加载器（T10，每请求按用户计算"最宽松"档，不缓存）。
	// nil = 不注入数据范围——member 端与未启用时的安全缺省：
	// gorm 插件只剩租户隔离，保持老行为。
	ScopeLoader tenant.ScopeLoader
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

		// 数据范围注入（T10）：仅管理端用户加载；member token 不加载（ScopeLoader 调用为 0）。
		// 失败方向 = 最严：Load 出错时按 Mode 5（仅本人）兜底注入，不阻断请求——
		// 加载失败不能退化成"看到更多数据"。
		if d.ScopeLoader != nil && claims.UserType == int(token.UserTypeAdmin) {
			ds := tenant.DataScope{Mode: 5, UserID: claims.UserID, DeptID: claims.DeptID} // 默认最严
			if loaded, err := d.ScopeLoader.Load(c.Request.Context(), claims.UserID, claims.DeptID); err == nil {
				ds = loaded
			}
			c.Request = c.Request.WithContext(tenant.WithDataScope(c.Request.Context(), ds))
		}

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
		claims, err := auth.ParseToken(tokenStr, d.Cfg)
		if err != nil {
			return nil, err
		}
		// 旧静态 JWT 仅管理端登录签发 → 恒为管理员类型（数据权限加载器依赖此值）。
		claims.UserType = int(token.UserTypeAdmin)
		return claims, nil
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
