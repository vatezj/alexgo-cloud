package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/pkg/audit"
	"alexGo-cloud/pkg/auth"
)

func OperateLogMiddleware(rec audit.OperateRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rec == nil {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api/admin/") {
			c.Next()
			return
		}
		if path == "/health" || path == "/metrics" || strings.HasPrefix(path, "/debug/pprof/") {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		claimsAny, _ := c.Get("claims")
		claims, _ := claimsAny.(*auth.Claims)
		if claims == nil {
			return
		}
		errMsg := ""
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}
		if len(errMsg) > 255 {
			errMsg = errMsg[:255]
		}
		_ = rec.RecordOperate(
			c.Request.Context(),
			claims.UserID,
			claims.Username,
			c.Request.Method,
			c.FullPath(),
			c.Writer.Status(),
			time.Since(start).Milliseconds(),
			errMsg,
		)
	}
}

