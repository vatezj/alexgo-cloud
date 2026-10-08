package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/tenant"
)

// NewTenantMiddleware 解析租户并注入 context，优先级：
// 1) 显式 X-Tenant-ID 头（App/开发直连）；
// 2) Host 域名匹配 tenants.domain（SaaS Web；lookup 为 nil 时跳过）；
// 3) 都没有 → 0（平台租户，历史行为）。
//
// lookup 出错按“未解析”处理（tid=0），不阻断请求——域名解析是增强能力，不是可用性依赖。
func NewTenantMiddleware(lookup tenant.DomainLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := tenant.ParseTenantID(c.GetHeader("X-Tenant-ID"))
		if id == 0 && lookup != nil {
			host := c.Request.Host
			if h, _, ok := strings.Cut(host, ":"); ok {
				host = h // 去端口再匹配
			}
			if resolved, err := lookup(c.Request.Context(), host); err == nil {
				id = resolved
			}
		}
		c.Request = c.Request.WithContext(tenant.WithTenantID(c.Request.Context(), id))
		c.Next()
	}
}
