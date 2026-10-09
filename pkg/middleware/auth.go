package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
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
		// 公开路径：/api/admin/** 需登录态；vben 自读三端点（isVbenSelfRead）走
		// "token→claims→租户→member 门槛"后放行（跳过 data_scope 与 Casbin，见锚点 2）；
		// 其余（含 /api/app/** 全部、/api/auth/login|logout、health/metrics/pprof）一律放行。
		// I1 取舍：member 的 refresh/logout 是 possession-based——凭 body/头自行校验，
		// 放进本中间件只会在 access 过期时把刷新链路也 401 拦死，故不列入鉴权清单。
		if !strings.HasPrefix(path, "/api/admin/") && !isVbenSelfRead(path) {
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

		// spec §4.3：Token 内租户为准——覆盖/回写 ctx，防 X-Tenant-ID 头伪造跨租户。
		// （公开路径无 claims，不经过此处，仍由 TenantMiddleware 按 header/domain 解析。）
		c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), claims.TenantID))

		// /api/admin/** 仅限管理员（C1）：member token（user_type=2）直接 403，
		// 且置于 Enforcer 之前——micro 形态 member-server 未装配 enforcer，此门槛
		// 在任何装配下都生效；即便 sub 撞名也进不了策略判定（I4）。
		if claims.UserType != int(token.UserTypeAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		// vben 自读档（锚点 2）：已过 token→claims→租户覆盖→member 门槛，
		// 到此必为管理员自读请求——跳过 data_scope 注入与 Casbin 直接放行。
		// /api/admin/** 不满足本条件，继续走下方完整门控，现行为不变。
		if !strings.HasPrefix(path, "/api/admin/") {
			c.Next()
			return
		}

		// 数据范围注入（T10）：仅管理端用户加载（上方门槛后必为管理员，条件保留作纵深防御）。
		// 失败方向 = 最严：Load 出错时按 Mode 5（仅本人）兜底注入，不阻断请求——
		// 加载失败不能退化成"看到更多数据"。
		if d.ScopeLoader != nil && claims.UserType == int(token.UserTypeAdmin) {
			ds := tenant.DataScope{Mode: 5, UserID: claims.UserID, DeptID: claims.DeptID} // 默认最严
			if loaded, err := d.ScopeLoader.Load(c.Request.Context(), claims.UserID, claims.DeptID); err == nil {
				ds = loaded
			} else if logger.Log != nil {
				// 静默降级会掩盖装配/DB 故障：兜底照做，但至少留一条告警。
				logger.Log.Warn("data scope load failed, fallback mode 5", zap.Error(err))
			}
			c.Request = c.Request.WithContext(tenant.WithDataScope(c.Request.Context(), ds))
		}

		if d.Enforcer != nil {
			// sub 带租户 + user_type 维度：{tenantId}:{userType}:{username}（C1）。
			// 只有 tenant 前缀时，member 昵称 "admin" 与管理员用户名撞 sub → g(x,x) 恒等提权；
			// user_type 维度让二者永不同 sub（空 username 退化为 {tid}:{ut}:{userid}）。
			sub := auth.UserSub(claims.TenantID, claims.UserType, claims.Username, claims.UserID)
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

// isVbenSelfRead：vben 前端启动期自读端点的精确清单（非前缀匹配——
// /api/auth/other、/api/auth/login 等必须留在公开档）。
// 这三个端点带 token 即代表登录态，授权语义由 handler 内按 claims 自查，
// 不需要 data_scope（不查业务行）也不走 Casbin（无对应路由策略）。
func isVbenSelfRead(path string) bool {
	switch path {
	case "/api/auth/codes", "/api/user/info", "/api/menu/all":
		return true
	}
	return false
}
