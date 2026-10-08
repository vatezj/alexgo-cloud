package admin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

type AuthController struct {
	permSvc service.PermissionService
	authSvc service.AuthService
}

func NewAuthController(permSvc service.PermissionService, authSvc service.AuthService) *AuthController {
	return &AuthController{permSvc: permSvc, authSvc: authSvc}
}

type profileResponse struct {
	User  *auth.Claims  `json:"user"`
	Roles []string      `json:"roles"`
	Perms []string      `json:"perms"`
	Menus any           `json:"menus"`
	Routes any          `json:"routes"`
}

func (c *AuthController) Profile(ctx *gin.Context) {
	claimsAny, ok := ctx.Get("claims")
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	claims, ok := claimsAny.(*auth.Claims)
	if !ok || claims == nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roles, err := c.permSvc.UserRoles(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	roleCodes := make([]string, 0, len(roles))
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
	}

	perms, err := c.permSvc.UserPermCodes(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	menus, err := c.permSvc.UserMenus(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	routes, err := c.permSvc.UserRoutes(ctx.Request.Context(), claims.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, profileResponse{
		User:  claims,
		Roles: roleCodes,
		Perms: perms,
		Menus: menus,
		Routes: routes,
	})
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh 用 refresh_token 换取新令牌对。
func (c *AuthController) Refresh(ctx *gin.Context) {
	var req refreshRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "refresh_token required"})
		return
	}
	res, err := c.authSvc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}

// Logout 注销当前 access token（从 Authorization 头取）。
func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	if err := c.authSvc.Logout(ctx.Request.Context(), raw); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
