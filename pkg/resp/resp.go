// Package resp 是唯一出站 HTTP 响应出口：所有 API 响应必须经 OK/Fail/Page。
// 豁免清单（勿改）：oauth2 protocol 端点、/health*、/metrics、/debug/pprof/**。
package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/logger"
)

type body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// statusFor 按错误码段映射 HTTP status：1→400，2→401（20003→403），3→400，5→500。
func statusFor(code errs.Code) int {
	switch {
	case code == errs.CodeForbidden:
		return http.StatusForbidden
	case code >= 10000 && code < 20000:
		return http.StatusBadRequest
	case code >= 20000 && code < 30000:
		return http.StatusUnauthorized
	case code >= 30000 && code < 40000:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func write(c *gin.Context, status int, code int, msg string, data any) {
	c.JSON(status, body{Code: code, Msg: msg, Data: data})
}

// OK 写成功信封；无返回值动作传 nil。
func OK(c *gin.Context, data any) {
	write(c, http.StatusOK, 0, "ok", data)
}

// Fail 写失败信封：*BizError 取码与文案；其他错误兜底 50000，原始错误只进日志不外泄。
// BizError 的 cause（gin 校验细节如字段名）同样只进日志：warn 带 cause + path，
// 响应体仍是 {code, msg: be.Msg(), data:null}，cause 绝不进 body。
func Fail(c *gin.Context, err error) {
	if err == nil {
		write(c, http.StatusInternalServerError, int(errs.CodeInternal), "系统繁忙，请稍后重试", nil)
		return
	}
	if be, ok := errs.As(err); ok {
		if cause := be.Unwrap(); cause != nil && logger.Log != nil {
			logger.Log.Warn("biz error", zap.Error(cause), zap.String("path", reqPath(c)))
		}
		write(c, statusFor(be.Code), int(be.Code), be.Msg(), nil)
		return
	}
	if logger.Log != nil {
		logger.Log.Error("unhandled error", zap.Error(err), zap.String("path", reqPath(c)))
	}
	write(c, http.StatusInternalServerError, int(errs.CodeInternal), "系统繁忙，请稍后重试", nil)
}

// reqPath 安全取请求路径：gin.CreateTestContext 构造的 c.Request 为 nil，
// 单测里直接解引用会 panic，故先判空。
func reqPath(c *gin.Context) string {
	if c.Request != nil {
		return c.Request.URL.Path
	}
	return ""
}
