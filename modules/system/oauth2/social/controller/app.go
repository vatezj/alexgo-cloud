package controller

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/oauth2/social"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// LoginBinder SocialService 的 app 端门面（controller 只依赖接口，测试可 stub）。
type LoginBinder interface {
	Login(ctx context.Context, platform, code, ip, ua string) (*oauth2svc.LoginResult, error)
	BindSocial(ctx context.Context, memberID uint64, platform, code string) error
	BindMobile(ctx context.Context, memberID uint64, mobile string) error
	UnbindSocial(ctx context.Context, memberID uint64, platform string) error
}

// AppController app 端第三方登录接口：两条公开登录 + 三条 Bearer 自鉴权绑定
// （不走 casbin；claims 模式与 scorecard controller/app 同构）。
type AppController struct {
	svc       LoginBinder
	validator token.Validator
}

func NewAppController(svc LoginBinder, validator token.Validator) *AppController {
	return &AppController{svc: svc, validator: validator}
}

// claims 解析 Bearer 并校验 member 身份；成功后把 token 内租户写回请求 ctx
// （TenantMiddleware 按 X-Tenant-ID 头解析的值在此被覆盖，伪造头无法跨租户）——
// 逐字复制 scorecard 的 app 端模式。失败已写响应，ok=false 直接 return；
// 全部受保护 handler 先过本函数再取 ctx.Request.Context()，故 service 恒拿 token 租户。
func (c *AppController) claims(ctx *gin.Context) (*token.Claims, bool) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	cl, err := c.validator.Validate(ctx.Request.Context(), raw)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return nil, false
	}
	if cl.UserType != token.UserTypeMember {
		resp.Fail(ctx, errs.New(errs.CodeForbidden, nil))
		return nil, false
	}
	ctx.Request = ctx.Request.WithContext(tenant.WithTenantID(ctx.Request.Context(), cl.TenantID))
	return cl, true
}

// unavailableBody 503 专用信封体（复制 member controller/app/member_auth.go）：
// resp.statusFor 无 503 码段（最接近 50000→500），而「签发/装配故障 → 503
// （不冒充 401）」契约与 member 登录同口径——信封三键手写对齐 resp.body，
// HTTP status 保持 503。
type unavailableBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// login 共享登录实现：bind JSON {code} → svc.Login（platform 由端点指定）。
// 失败映射：签发链路故障（ErrIssuance / ErrAuthenticatorMissing）→ 503 信封
// code 50000 "auth service unavailable"（I2 同 member 登录）；
// 其余（3030x 业务码 / 裸 error）→ resp.Fail 统一信封。
func (c *AppController) login(ctx *gin.Context, platform string) {
	var req struct {
		Code string `json:"code"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.Code == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	res, err := c.svc.Login(ctx.Request.Context(), platform, req.Code,
		ctx.ClientIP(), ctx.GetHeader("User-Agent"))
	if err != nil {
		if errors.Is(err, oauth2svc.ErrIssuance) || errors.Is(err, oauth2svc.ErrAuthenticatorMissing) {
			ctx.JSON(http.StatusServiceUnavailable, unavailableBody{
				Code: int(errs.CodeInternal), Msg: "auth service unavailable", Data: nil,
			})
			return
		}
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, res)
}

// LoginMP POST /api/app/social/wechat/mp/login（公开：微信小程序 code2session 登录）。
func (c *AppController) LoginMP(ctx *gin.Context) {
	c.login(ctx, social.PlatformWechatMP)
}

// LoginWeb POST /api/app/social/wechat/web/login（公开：微信网页授权登录）。
func (c *AppController) LoginWeb(ctx *gin.Context) {
	c.login(ctx, social.PlatformWechatWeb)
}

// BindSocial POST /api/app/member/auth/bind-social {platform, code}（Bearer）。
func (c *AppController) BindSocial(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		Platform string `json:"platform"`
		Code     string `json:"code"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.Platform == "" || req.Code == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.BindSocial(ctx.Request.Context(), cl.UserID, req.Platform, req.Code); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// BindMobile POST /api/app/member/auth/bind-mobile {mobile}（Bearer）。
func (c *AppController) BindMobile(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	var req struct {
		Mobile string `json:"mobile"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	if req.Mobile == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.BindMobile(ctx.Request.Context(), cl.UserID, req.Mobile); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}

// UnbindSocial DELETE /api/app/member/auth/social/:platform（Bearer，平台取路径参数）。
func (c *AppController) UnbindSocial(ctx *gin.Context) {
	cl, ok := c.claims(ctx)
	if !ok {
		return
	}
	platform := ctx.Param("platform")
	if platform == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, nil))
		return
	}
	if err := c.svc.UnbindSocial(ctx.Request.Context(), cl.UserID, platform); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
