package app

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/member/service"
)

// AuthController app 端会员鉴权接口（register/login 公开；refresh/logout 需登录态）。
type AuthController struct {
	svc service.MemberService
}

// NewAuthController 构造控制器（fx 注入 MemberService）。
func NewAuthController(svc service.MemberService) *AuthController { return &AuthController{svc: svc} }

type registerRequest struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type loginRequest struct {
	Mobile   string `json:"mobile"`
	Password string `json:"password"`
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

// Login 会员登录（公开接口），成功返回 access/refresh token。
func (c *AuthController) Login(ctx *gin.Context) {
	var req loginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		errJSON(ctx, http.StatusBadRequest, err)
		return
	}
	res, err := c.svc.Login(ctx.Request.Context(), req.Mobile, req.Password, ctx.ClientIP())
	if err != nil {
		errJSON(ctx, http.StatusUnauthorized, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

// Refresh 刷新令牌（需登录态：中间件 appAuthRequired 清单）。
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

// Logout 注销当前 access token（需登录态：中间件 appAuthRequired 清单）。
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
