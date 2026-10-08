package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"alexGo-cloud/pkg/logger"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if logger.Log != nil {
			logger.Log.Info("incoming request",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
			)
		}
		c.Next()
	}
}

func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}

func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
