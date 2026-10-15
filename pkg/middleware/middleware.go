package middleware

import (
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	errs "alexGo-cloud/pkg/errors"
	pkglogger "alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/resp"
)

// Recovery 统一 panic 兜底：zap.Error 落 panic 值 + path + 调用栈，
// 再写 {code:50000} 标准信封（HTTP 500）。
// logger 取用时回落 pkglogger.Log（同 AccessLog：loggerForTest 在包
// 初始化期拷贝，生产恒为 nil，须调用时回落）。
// 顺序注意：必须先 resp.Fail 写信封、再 c.Abort()
// （Abort 后 gin 拒绝再写 body）。
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log := loggerForTest
				if log == nil {
					log = pkglogger.Log
				}
				if log != nil {
					log.Error("panic recovered",
						zap.Any("panic", rec),
						zap.String("path", c.Request.URL.Path),
						zap.ByteString("stack", debug.Stack()))
				}
				resp.Fail(c, errs.New(errs.CodeInternal, nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}

func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
