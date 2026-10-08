package middleware

import (
	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/tenant"
)

// TenantMiddleware 从请求 Header 提取租户信息并注入到 context。
//
// 约定：
// - Header：X-Tenant-ID: <uint64>
//
// 使用场景：
// - SaaS 多租户数据隔离（在 Repository/Service 层按 tenant_id 过滤）
// - 多租户限流维度（tenant + ip + path）
// - 审计日志（记录租户维度）
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := tenant.ParseTenantID(c.GetHeader("X-Tenant-ID"))
		c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), id))
		c.Next()
	}
}
