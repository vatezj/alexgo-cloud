package monitor

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

// RequestTotal 记录 HTTP 请求总量（Counter）。
//
// label 维度：
// - method：GET/POST/...
// - path：Gin FullPath（路由模板），比 URL.Path 更适合聚合统计
// - status：http status code（字符串）
//
// 指标用途：
// - 错误率：sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
// - 热点接口：topk(10, sum(rate(http_requests_total[1m])) by (path))
var RequestTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	},
	[]string{"method", "path", "status"},
)

func init() {
	// init 注册指标，Prometheus Handler 会自动暴露注册表中的指标。
	prometheus.MustRegister(RequestTotal)
}

// PrometheusMiddleware 在请求完成后统计一次请求。
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		status := c.Writer.Status()
		RequestTotal.WithLabelValues(c.Request.Method, c.FullPath(), fmt.Sprintf("%d", status)).Inc()
	}
}
