package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"alexGo-cloud/pkg/auth"
	pkglogger "alexGo-cloud/pkg/logger"
)

// loggerForTest 供单测替换 observer；生产始终用 pkglogger.Log。
// 注意：Go 包初始化先于 main()，此时 logger.Init() 尚未执行，
// 直接在此赋值只会拷到 nil；故取用时若 loggerForTest 为 nil 则回落 pkglogger.Log。
var loggerForTest = pkglogger.Log

func accessLogSkip(path string) bool {
	switch path {
	case "/health", "/health/ready", "/metrics":
		return true
	}
	return len(path) >= len("/debug/pprof/") && path[:len("/debug/pprof/")] == "/debug/pprof/"
}

// AccessLog 结构化访问日志。必须挂在链尾（OTEL 之后、Auth 之后），
// 否则 trace_id/user_id 恒空。
// 注意：Auth/RateLimit 的中止路径（Abort 短路）到不了链尾，由二者在中止点
// 直接调 logAccess 补记（终审方案②——链顺序不动，见 alexgo-server/server/http.go）。
func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		if accessLogSkip(c.Request.URL.Path) {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		logAccess(c, time.Since(start))
	}
}

// logAccess 输出一条访问日志：字段、级别（≥500 Error / ≥400 Warn / 其余 Info）
// 与 skip 清单与 AccessLog 原行为一致，供 AccessLog 链尾与 Auth/RateLimit 中止点共用。
// latency 由调用方给出（链尾 = Next 前后差；中止点 = 中间件入口起算）。
// claims/trace_id 尽力取：中止点早于 OTEL → 无 trace_id；401 无 claims → 无 user_id，
// 缺失即缺省，不编造。
func logAccess(c *gin.Context, latency time.Duration) {
	if accessLogSkip(c.Request.URL.Path) {
		return
	}
	log := loggerForTest
	if log == nil {
		log = pkglogger.Log
	}
	if log == nil {
		return
	}
	fields := []zap.Field{
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("query", truncateQuery(c.Request.URL.RawQuery, 2048)),
		zap.Int("status", c.Writer.Status()),
		zap.Int64("latency_ms", latency.Milliseconds()),
		zap.String("client_ip", c.ClientIP()),
	}
	if cl, _ := c.Get("claims"); cl != nil {
		if claims, ok := cl.(*auth.Claims); ok && claims != nil {
			fields = append(fields,
				zap.Uint64("user_id", claims.UserID),
				zap.Uint64("tenant_id", claims.TenantID))
		}
	}
	if sc := trace.SpanFromContext(c.Request.Context()).SpanContext(); sc.HasTraceID() {
		fields = append(fields, zap.String("trace_id", sc.TraceID().String()))
	}
	status := c.Writer.Status()
	switch {
	case status >= 500:
		log.Error("http request", fields...)
	case status >= 400:
		log.Warn("http request", fields...)
	default:
		log.Info("http request", fields...)
	}
}

func truncateQuery(q string, max int) string {
	if len(q) <= max {
		return q
	}
	return q[:max] + "..."
}
