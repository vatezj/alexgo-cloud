package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
)

// AuthMiddleware 是“管理端接口”的统一鉴权入口。
//
// 约定：
// - 仅对 /api/admin/** 启用鉴权（避免影响公开接口、健康检查、指标等）。
// - 鉴权拆成两步：JWT 身份校验 + (可选) Casbin 权限校验。
//
// Header 约定：
// - Authorization: Bearer <jwt>
//
// Context 注入：
// - claims：解析后的 *auth.Claims（用户名、用户ID、租户ID等）
func AuthMiddleware(cfg *config.Config, enforcer *casbin.Enforcer) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 基础运维接口：默认放行（生产可结合网关/内网隔离）。
		if path == "/health" || path == "/metrics" || strings.HasPrefix(path, "/debug/pprof/") {
			c.Next()
			return
		}
		// 非 admin API：默认放行（例如 app 登录、公开接口等）。
		if !strings.HasPrefix(path, "/api/admin/") {
			c.Next()
			return
		}

		// 1) JWT：校验 token 是否有效，并解析用户身份。
		tokenStr := c.GetHeader("Authorization")
		claims, err := auth.ParseToken(tokenStr, cfg)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("claims", claims)

		// 2) Casbin（可选）：如果提供了 enforcer，则按 sub/obj/act 判断是否允许访问。
		// sub：用户名（缺省则用 user_id 字符串兜底）
		// obj：路由模板（FullPath），避免被 path 参数干扰；FullPath 为空则回退实际 URL path。
		// act：HTTP method
		if enforcer != nil {
			sub := claims.Username
			if sub == "" {
				sub = fmt.Sprintf("%d", claims.UserID)
			}
			obj := c.FullPath()
			if obj == "" {
				obj = path
			}
			act := c.Request.Method
			ok, eerr := enforcer.Enforce(sub, obj, act)
			if eerr != nil || !ok {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		// 通过鉴权，继续执行后续 handler。
		c.Next()
	}
}
