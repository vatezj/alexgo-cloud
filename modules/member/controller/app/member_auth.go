package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

// AuthController app 端会员鉴权接口（register/login/refresh/logout 全部公开入口：
// refresh/logout 是 possession-based，由 controller 持有的 token 本身鉴权）。
type AuthController struct {
	svc     service.MemberService
	unified oauth2svc.UnifiedLogin
}

// NewAuthController 构造控制器（fx 注入 MemberService + 统一登录门面）。
func NewAuthController(svc service.MemberService, unified oauth2svc.UnifiedLogin) *AuthController {
	return &AuthController{svc: svc, unified: unified}
}

type registerRequest struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Mobile      string `json:"mobile"`
	Password    string `json:"password"`
	CaptchaUUID string `json:"captcha_uuid"`
	CaptchaCode string `json:"captcha_code"`
}

// unavailableBody 503 专用信封体：resp.statusFor 无 503 码段（最接近 50000→500），
// 而 I2 的「签发/装配故障 → 503（不冒充 401）」契约钉在 member/cmd/main.go、
// service/member.go 注释与既有测试里——信封三键手写对齐 resp.body，HTTP status 保持 503。
type unavailableBody struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// Register godoc
//
//	@Summary		会员注册（公开）
//	@Description	body {mobile,password,nickname}；查重 → 30009、字段校验 → 10001。响应 pkg/resp 信封，data 为会员行。免鉴权。
//	@Tags			member-app
//	@Accept			json
//	@Produce		json
//	@Param			body	body	object	true	"注册信息"	Example({"mobile": "13800138000", "password": "secret", "nickname": "阿飞"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/app/member/auth/register [post]
func (c *AuthController) Register(ctx *gin.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	u, err := c.svc.Register(ctx.Request.Context(), req.Mobile, req.Password, req.Nickname, ctx.ClientIP())
	if err != nil {
		// service 已哨兵化：查重 → 30009、字段校验 → 10001（BizError 语义码经 resp.Fail 直传）；
		// 其余（额度/bcrypt/落库）裸 error → 兜底 50000（原始错误只进日志不外泄）。
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, u)
}

// Login godoc
//
//	@Summary		会员登录（公开）
//	@Description	统一登录（account_type=member，client=alexgo-app），带 captcha 两段。data={token,refresh_token,expires_in}。失败：captcha 拒 → 400/30005；签发/装配故障 → 503/50000；其余 401/20001 固定文案（防枚举）。免鉴权。
//	@Tags			member-app
//	@Accept			json
//	@Produce		json
//	@Param			body	body	object	true	"登录信息"	Example({"mobile": "13800138000", "password": "secret", "captcha_uuid": "uuid", "captcha_code": "abcd"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Failure		503		{object}	map[string]interface{}
//	@Router			/app/member/auth/login [post]
func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	res, _, err := c.unified.Login(ctx.Request.Context(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeMember,
		Username:    req.Mobile,
		Password:    req.Password,
		ClientID:    "alexgo-app",
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          ctx.ClientIP(),
		UserAgent:   ctx.GetHeader("User-Agent"),
	})
	if err != nil {
		switch {
		case errors.Is(err, oauth2svc.ErrIssuance), errors.Is(err, oauth2svc.ErrAuthenticatorMissing):
			// 503 状态保持（I2），信封文案沿用旧字面量（内部故障描述，非原始错误串）。
			ctx.JSON(http.StatusServiceUnavailable, unavailableBody{
				Code: int(errs.CodeInternal), Msg: "auth service unavailable", Data: nil,
			})
		case errors.Is(err, oauth2svc.ErrCaptchaInvalid):
			resp.Fail(ctx, errs.New(errs.CodeCaptchaInvalid, nil))
		default:
			resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		}
		return
	}
	// token 三元组移入 data（LoginResult json 标签即 token/refresh_token/expires_in，键名逐字不变）。
	resp.OK(ctx, res)
}

// Refresh godoc
//
//	@Summary		刷新令牌
//	@Description	possession-based：凭 body 里的 refresh_token 刷新，无须登录态中间件（access 过期正是刷新场景）。失败 401。免 X-Session。
//	@Tags			member-app
//	@Accept			json
//	@Produce		json
//	@Param			body	body	object	true	"刷新令牌"	Example({"refresh_token": "..."})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/app/member/auth/refresh [post]
func (c *AuthController) Refresh(ctx *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	// bind 成功但 token 为空：显式缺参拒绝（旧代码在此防 errJSON 收到 nil panic）。
	if req.RefreshToken == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	res, err := c.svc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	resp.OK(ctx, res)
}

// Logout godoc
//
//	@Summary		注销当前 access token
//	@Description	possession-based：凭 Authorization 头的 access token 自行撤销，无须登录态中间件（I1）。
//	@Tags			member-app
//	@Produce		json
//	@Security		Bearer
//	@Success		200	{object}	map[string]interface{}
//	@Failure		400	{object}	map[string]interface{}
//	@Router			/app/member/auth/logout [post]
func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	if err := c.svc.Logout(ctx.Request.Context(), raw); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
