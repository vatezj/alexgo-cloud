package middleware

import (
	"errors"
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
		// 运维端点不参与限流/熔断：/health 与 /health/ready 必须在依赖故障期间保持可用，
		// 否则熔断打开会把 liveness 打成 503，触发 K8s 全员重启（防雪崩被击穿）。
		// 同理这些端点也豁免限流（limiter.enabled=true 时同样直接放行），与 AuthMiddleware 的
		// 运维端点放行约定一致：探针与指标抓取不该被业务侧的流量/故障状态误伤。
		path := c.Request.URL.Path
		if path == "/health" || path == "/health/ready" || path == "/metrics" {
			c.Next()
			return
		}

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
			err := cb.Do(func() error {
				c.Next()
				if c.Writer.Status() >= 500 {
					return fmt.Errorf("server error")
				}
				return nil
			})
			if errors.Is(err, circuitbreaker.ErrOpen) {
				// 熔断 Open / 探测位被占用：Do 未执行 fn（c.Next() 未被调用），必须显式 Abort，
				// 否则 gin 的外层 handler 循环会继续执行下游路由、把请求照样放行。
				// 不能靠 c.Writer.Status()==0 判断——gin 的 writer 在请求开始时初始 status 即 200。
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			if c.IsAborted() {
				return
			}
			return
		}

		c.Next()
	}
}
