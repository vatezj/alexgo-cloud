package errors

import (
	"errors"
	"fmt"
)

// Code 五位业务错误码：1xxxx 参数 / 2xxxx 认证 / 3xxxx 业务 / 5xxxx 系统。
type Code int

const (
	// 1xxxx 参数
	CodeParamInvalid Code = 10001
	CodeParamMissing Code = 10002

	// 2xxxx 认证授权
	CodeUnauthorized Code = 20001
	CodeTokenExpired Code = 20002
	CodeForbidden    Code = 20003

	// 3xxxx 业务规则
	CodeUsernameTaken  Code = 30001
	CodeUserNotFound   Code = 30002
	CodeRoleTaken      Code = 30003
	CodeTenantDisabled Code = 30004
	CodeCaptchaInvalid Code = 30005
	CodeNoticeNotFound Code = 30006

	// 5xxxx 系统
	CodeInternal Code = 50000
)

// registry 是错误码文案表（中文、写死代码，不做 i18n）。新码追加到这里 + 顶部常量。
var registry = map[Code]string{
	CodeParamInvalid:   "参数校验失败",
	CodeParamMissing:   "缺少必填参数",
	CodeUnauthorized:   "未登录或登录已失效",
	CodeTokenExpired:   "登录已过期",
	CodeForbidden:      "无权限",
	CodeUsernameTaken:  "用户名已存在",
	CodeUserNotFound:   "用户不存在",
	CodeRoleTaken:      "角色标识已存在",
	CodeTenantDisabled: "租户已停用",
	CodeCaptchaInvalid: "验证码错误",
	CodeNoticeNotFound: "公告不存在",
	CodeInternal:       "系统繁忙，请稍后重试",
}

// BizError 携带业务错误码的错误。cause 可为 nil；cause 只进日志，禁止外泄给客户端。
type BizError struct {
	Code  Code
	cause error
}

func New(code Code, cause error) *BizError {
	return &BizError{Code: code, cause: cause}
}

func (e *BizError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Msg(), e.cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Msg())
}

func (e *BizError) Msg() string {
	if m, ok := registry[e.Code]; ok {
		return m
	}
	return registry[CodeInternal]
}

func (e *BizError) Unwrap() error { return e.cause }

// As 从错误链提取 *BizError。
func As(err error) (*BizError, bool) {
	var be *BizError
	if errors.As(err, &be) {
		return be, true
	}
	return nil, false
}

func Is(err error, code Code) bool {
	be, ok := As(err)
	return ok && be.Code == code
}
