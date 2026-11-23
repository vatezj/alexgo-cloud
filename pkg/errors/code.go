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
	// CodeScorecardInOtherRoom scorecard：同一时间只能在一个进行中牌局（仅进房 Join 拦截；
	// 开房 Create 对已有局复用返回该局快照，不再触发本码）。
	CodeScorecardInOtherRoom Code = 30114
	// CodeImpersonateMemberNotFound online：代登录目标会员不存在/已删除/已停用。
	CodeImpersonateMemberNotFound Code = 30201
	// CodeCannotKickSelf online：不能踢出自己的会话。
	CodeCannotKickSelf Code = 30202
	// CodeSessionNotFound online：在线会话不存在（或已非管理员会话）。
	CodeSessionNotFound Code = 30203
	// ---- 303xx 第三方登录（social） ----
	// CodeSocialAppNotConfigured social：该租户未配置此登录渠道（或已停用）。
	CodeSocialAppNotConfigured Code = 30301
	// CodeSocialExchangeFailed social：第三方授权换取失败。
	CodeSocialExchangeFailed Code = 30302
	// CodeSocialIdentityBound social：该第三方身份已绑定其他账号。
	CodeSocialIdentityBound Code = 30303
	// CodeSocialMobileBound social：当前账号已绑定手机号。
	CodeSocialMobileBound Code = 30304
	// CodeSocialMergeRefused social：手机号已被占用，当前账号不满足自动合并条件。
	CodeSocialMergeRefused Code = 30305
	// CodeSocialUnbindLast social：解绑后将失去唯一登录方式。
	CodeSocialUnbindLast Code = 30306

	// ---- 304xx fishgame ----
	// CodeFishgameFishNotFound fishgame：图鉴条目不存在（查无/跨租户）。
	CodeFishgameFishNotFound Code = 30401
	// CodeFishgameFishReferenced fishgame：条目已被进度或事件引用，不可删除（引导改启停）。
	CodeFishgameFishReferenced Code = 30402
	// CodeFishgameFishKingInvalid fishgame：鱼王指定的条目不存在或已停用。
	CodeFishgameFishKingInvalid Code = 30403
	// CodeFishgameConfigNotFound fishgame：运营配置不存在（查无/跨租户）。
	CodeFishgameConfigNotFound Code = 30404

	// ---- 305xx starfuyu ----
	// CodeStarfuyuFortuneNotFound starfuyu：指定日期/星座无运势行（admin 详情与 my 类接口）。
	CodeStarfuyuFortuneNotFound Code = 30501
	// CodeStarfuyuImageFailed starfuyu：图片生成失败（gg 绘制或背景图/字体资源缺失）。
	CodeStarfuyuImageFailed Code = 30502
	// CodeStarfuyuLimitExceeded starfuyu：今日请求次数已达上限（bazi/divination 限流命中）。
	CodeStarfuyuLimitExceeded Code = 30503
	// CodeStarfuyuBaziNotFound starfuyu：尚未保存生辰八字（GET /bazi/my 无数据）。
	CodeStarfuyuBaziNotFound Code = 30504
	// CodeStarfuyuConstellationMissing starfuyu：尚未设置星座（fortune/my/* 需要 profile）。
	CodeStarfuyuConstellationMissing Code = 30505
	// CodeStarfuyuFetchFailed starfuyu：运势抓取失败（DeepSeek 返回不合法/超时）。
	CodeStarfuyuFetchFailed Code = 30506
	// CodeStarfuyuMpUploadFailed starfuyu：公众号上传失败（access_token / 草稿箱 API 错误）。
	CodeStarfuyuMpUploadFailed Code = 30507

	// ---- 306xx job ----
	// CodeJobNotFound pkg/job：手动触发的任务名未注册。
	CodeJobNotFound Code = 30601
	// CodeJobDisabled pkg/job：任务已停用（模块开关关 / 配置空串），拒绝手动触发。
	CodeJobDisabled Code = 30602

	// ---- 307xx taokouling ----
	// CodeTaoKoulingPasswordNotFound taokouling：分享文本里未识别到淘口令。
	CodeTaoKoulingPasswordNotFound Code = 30701
	// CodeTaoKoulingConfigMissing taokouling：淘口令服务配置缺失。
	CodeTaoKoulingConfigMissing Code = 30702
	// CodeTaoKoulingChannelMissing taokouling：未配置可用推广渠道。
	CodeTaoKoulingChannelMissing Code = 30703
	// CodeTaoKoulingRateLimited taokouling：请求过于频繁，限流命中。
	CodeTaoKoulingRateLimited Code = 30704
	// CodeTaoKoulingParseFailed taokouling：淘口令解析失败。
	CodeTaoKoulingParseFailed Code = 30705

	// 5xxxx 系统
	CodeInternal Code = 50000
	// CodeWxacodeNotConfigured scorecard 小程序码：租户未配置（或已停用）微信小程序渠道，
	// 无法生成小程序码（服务端配置缺失 → HTTP 500 + 明确 msg，前端降级房号展示）。
	CodeWxacodeNotConfigured Code = 50001
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
	CodeScorecardInOtherRoom:     "你还在其他牌局中，请先退出",

	CodeImpersonateMemberNotFound: "目标会员不存在或不可用",
	CodeCannotKickSelf:            "不能踢出自己的会话",
	CodeSessionNotFound:           "会话不存在",

	CodeSocialAppNotConfigured: "该租户未配置此登录渠道（或已停用）",
	CodeSocialExchangeFailed:   "第三方授权换取失败",
	CodeSocialIdentityBound:    "该第三方身份已绑定其他账号",
	CodeSocialMobileBound:      "当前账号已绑定手机号",
	CodeSocialMergeRefused:     "手机号已被占用，当前账号不满足自动合并条件",
	CodeSocialUnbindLast:       "解绑后将失去唯一登录方式",

	CodeFishgameFishNotFound:    "图鉴条目不存在",
	CodeFishgameFishReferenced:  "条目已被进度或事件引用，不可删除",
	CodeFishgameFishKingInvalid: "鱼王指定的条目不存在或已停用",
	CodeFishgameConfigNotFound:  "运营配置不存在",

	CodeStarfuyuFortuneNotFound:      "运势数据不存在",
	CodeStarfuyuImageFailed:          "图片生成失败",
	CodeStarfuyuLimitExceeded:        "今日请求次数已达上限",
	CodeStarfuyuBaziNotFound:         "尚未保存生辰八字",
	CodeStarfuyuConstellationMissing: "尚未设置星座",
	CodeStarfuyuFetchFailed:          "运势抓取失败",
	CodeStarfuyuMpUploadFailed:       "公众号上传失败",

	CodeJobNotFound: "任务不存在",
	CodeJobDisabled: "任务已停用",

	CodeTaoKoulingPasswordNotFound: "未识别到淘口令",
	CodeTaoKoulingConfigMissing:    "淘口令服务配置缺失",
	CodeTaoKoulingChannelMissing:   "未配置可用推广渠道",
	CodeTaoKoulingRateLimited:      "请求过于频繁，请稍后再试",
	CodeTaoKoulingParseFailed:      "淘口令解析失败",

	CodeInternal: "系统繁忙，请稍后重试",

	CodeWxacodeNotConfigured: "微信小程序渠道未配置，无法生成小程序码",
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
