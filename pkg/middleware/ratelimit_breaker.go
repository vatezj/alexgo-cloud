package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/circuitbreaker"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/limiter"
	"alexGo-cloud/pkg/tenant"
)

// RateLimitAndBreaker 是入口防护中间件：分布式限流 + 熔断（可按配置开关）。
//
// 触发点：
// - limiter.enabled=true 时启用 Redis Token Bucket（429）。
// - breaker.enabled=true 时启用熔断器：
//   - 当连续出现 5xx（由 handler 写出的状态码）达到阈值后进入 Open（503）。
//
// 限流 key 设计：
// - 这里使用 tenant + clientIP + FullPath 组合，能够在 SaaS 多租户下实现隔离限流。
// - FullPath 为空时会变成空字符串，实际可按需回退 URL.Path（可继续优化）。
func RateLimitAndBreaker(cfg *config.Config, rl *limiter.RateLimiter, cb *circuitbreaker.CircuitBreaker) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg != nil && cfg.Limiter.Enabled {
			tid := tenant.TenantIDFromContext(c.Request.Context())
			key := fmt.Sprintf("%d:%s:%s", tid, c.ClientIP(), c.FullPath())
			if !rl.Allow(c.Request.Context(), key, cfg.Limiter.Rate, cfg.Limiter.Burst) {
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}

		if cfg != nil && cfg.Breaker.Enabled {
			// 这里用 c.Next() 执行后，根据响应码判断是否“失败”。
			// 如果你希望按错误类型更精细地统计（例如 only DB errors），可以把 Do 包裹在下游调用处，而不是 HTTP 全局层。
			_ = cb.Do(func() error {
				c.Next()
				if c.Writer.Status() >= 500 {
					return fmt.Errorf("server error")
				}
				return nil
			})
			if c.IsAborted() {
				return
			}
			if c.Writer.Status() == 0 {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			return
		}

		c.Next()
	}
}
