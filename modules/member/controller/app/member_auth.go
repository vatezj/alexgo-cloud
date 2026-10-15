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

// Register 注册会员（公开接口）。
func (c *AuthController) Register(ctx *gin.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		return
	}
	u, err := c.svc.Register(ctx.Request.Context(), req.Mobile, req.Password, req.Nickname, ctx.ClientIP())
	if err != nil {
		// service 层为裸 fmt.Errorf（service 不在本任务改动清单）→ 按错误字面量桥语义码
		// （Task 8 user.go「already exists」先例，fail-safe 兜底 50000）：
		// 查重「already registered」→ 30009 手机号已存在；字段校验 → 10001 参数校验失败；
		// 其余（额度/bcrypt/落库）裸透传 resp.Fail → 50000（原始错误只进日志不外泄）。
		switch {
		case strings.Contains(err.Error(), "already registered"):
			resp.Fail(ctx, errs.New(errs.CodeMobileTaken, nil))
		case strings.Contains(err.Error(), "invalid mobile"),
			strings.Contains(err.Error(), "password too short"),
			strings.Contains(err.Error(), "must not contain ':'"):
			resp.Fail(ctx, errs.New(errs.CodeParamInvalid, err))
		default:
			resp.Fail(ctx, err)
		}
		return
	}
	resp.OK(ctx, u)
}

// Login 会员登录（公开接口）——内部走统一登录（account_type=member，client=alexgo-app）。
// 映射：签发/装配故障 → 503 信封 code 50000（I2 不冒充 401）；captcha 拒绝 → 30005 400；
// 其余 → 20001 固定文案（vben 登录同构，防枚举）。
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

// Refresh 刷新令牌。possession-based：凭 body 里的 refresh_token，无须中间件登录态
// （access 过期正是刷新的场景——见 I1，中间件不再拦 /api/app/**）。
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

// Logout 注销当前 access token。possession-based：凭 Authorization 头的 access token
// 自行撤销（无须中间件登录态，I1）。
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
