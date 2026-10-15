package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
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

func errJSON(c *gin.Context, code int, err error) {
	c.JSON(code, gin.H{"error": err.Error()})
}

// Register 注册会员（公开接口）。
func (c *AuthController) Register(ctx *gin.Context) {
	var req registerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	u, err := c.svc.Register(ctx.Request.Context(), req.Mobile, req.Password, req.Nickname, ctx.ClientIP())
	if err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": u})
}

// Login 会员登录（公开接口）——内部走统一登录（account_type=member，client=alexgo-app）。
// 映射：签发/装配故障 → 503（不冒充 401）；captcha 拒绝 → 400；其余 → 401。
func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
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
			errJSON(ctx, http.StatusServiceUnavailable, fmt.Errorf("auth service unavailable"))
		case errors.Is(err, oauth2svc.ErrCaptchaInvalid):
			errJSON(ctx, http.StatusBadRequest, err)
		default:
			errJSON(ctx, http.StatusUnauthorized, err)
		}
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

// Refresh 刷新令牌。possession-based：凭 body 里的 refresh_token，无须中间件登录态
// （access 过期正是刷新的场景——见 I1，中间件不再拦 /api/app/**）。
func (c *AuthController) Refresh(ctx *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	// bind 成功但 token 为空：err 为 nil，须显式报错（否则 errJSON 收到 nil panic）。
	if req.RefreshToken == "" {
		errJSON(ctx, http.StatusBadRequest, fmt.Errorf("refresh_token required"))
		return
	}
	res, err := c.svc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		errJSON(ctx, http.StatusUnauthorized, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

// Logout 注销当前 access token。possession-based：凭 Authorization 头的 access token
// 自行撤销（无须中间件登录态，I1）。
func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		errJSON(ctx, http.StatusBadRequest, fmt.Errorf("token required"))
		return
	}
	if err := c.svc.Logout(ctx.Request.Context(), raw); err != nil {
		errJSON(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
