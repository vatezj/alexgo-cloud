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
	// CodeCodegenTableNotFound codegen：配置表/元数据表不存在（导入/详情/同步）。
	CodeCodegenTableNotFound Code = 30007
	// CodeCodegenTableTaken codegen：表已导入（重复导入去重拒绝）。
	CodeCodegenTableTaken Code = 30008
	// CodeMobileTaken member：手机号已注册（注册查重拒绝）。
	CodeMobileTaken Code = 30009
	// CodeFileNotFound file：按 id/key 查无；public 读 private 行也归此（不泄露存在性）。
	CodeFileNotFound Code = 30010
	// CodeFileTypeNotAllowed file：扩展名不在白名单或探测 mime 与扩展名不符。
	CodeFileTypeNotAllowed Code = 30011
	// CodeFileTooLarge file：超过 storage.max_size_mb 上限。
	CodeFileTooLarge Code = 30012
	// CodeExportTooManyRows excel：导出行数超过 MaxExportRows 上限（5万保险丝）。
	CodeExportTooManyRows Code = 30013
	// CodeScorecardRoomNotFound scorecard：房间不存在或非本租户。
	CodeScorecardRoomNotFound Code = 30101
	// CodeScorecardNotMember scorecard：非房间成员。
	CodeScorecardNotMember Code = 30102
	// CodeScorecardRoomClosed scorecard：房间已关闭。
	CodeScorecardRoomClosed Code = 30103
	// CodeScorecardRoomSettled scorecard：房间已结算，禁止记分。
	CodeScorecardRoomSettled Code = 30104
	// CodeScorecardNotOwner scorecard：仅房主可操作。
	CodeScorecardNotOwner Code = 30105
	// CodeScorecardRoomTaken scorecard：房号生成冲突重试耗尽。
	CodeScorecardRoomTaken Code = 30106
	// CodeScorecardTransferInvalid scorecard：转让目标无效（不在房/拉黑/同人）。
	CodeScorecardTransferInvalid Code = 30107
	// CodeScorecardBlacklisted scorecard：存在拉黑关系，禁止进房/加好友。
	CodeScorecardBlacklisted Code = 30108
	// CodeScorecardFriendTaken scorecard：好友关系已存在。
	CodeScorecardFriendTaken Code = 30109
	// CodeScorecardRecordNotFound scorecard：战绩不存在。
	CodeScorecardRecordNotFound Code = 30110
	// CodeScorecardNotYourRecord scorecard：仅能操作本人战绩。
	CodeScorecardNotYourRecord Code = 30111
	// CodeScorecardVersionConflict scorecard：并发冲突，请刷新快照重试。
	CodeScorecardVersionConflict Code = 30112
	// CodeScorecardTargetNotMember scorecard：目标不是房间成员。
	CodeScorecardTargetNotMember Code = 30113

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

	CodeCodegenTableNotFound: "数据表不存在",
	CodeCodegenTableTaken:    "数据表已导入",
	CodeMobileTaken:          "手机号已存在",

	CodeFileNotFound:       "文件不存在",
	CodeFileTypeNotAllowed: "文件类型不允许",
	CodeFileTooLarge:       "文件超过大小限制",

	CodeExportTooManyRows: "导出行数超过上限",

	CodeScorecardRoomNotFound:    "房间不存在",
	CodeScorecardNotMember:       "你不是该房间成员",
	CodeScorecardRoomClosed:      "房间已关闭",
	CodeScorecardRoomSettled:     "房间已结算",
	CodeScorecardNotOwner:        "仅房主可操作",
	CodeScorecardRoomTaken:       "房号生成失败，请重试",
	CodeScorecardTransferInvalid: "转让目标无效",
	CodeScorecardBlacklisted:     "存在拉黑关系，无法操作",
	CodeScorecardFriendTaken:     "好友关系已存在",
	CodeScorecardRecordNotFound:  "战绩不存在",
	CodeScorecardNotYourRecord:   "仅能操作本人战绩",
	CodeScorecardVersionConflict: "操作冲突，请下拉刷新",
	CodeScorecardTargetNotMember: "目标不是房间成员",

	CodeInternal: "系统繁忙，请稍后重试",
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
