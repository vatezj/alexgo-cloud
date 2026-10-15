package admin

import (
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

type AuthController struct {
	permSvc service.PermissionService
	authSvc service.AuthService
}

func NewAuthController(permSvc service.PermissionService, authSvc service.AuthService) *AuthController {
	return &AuthController{permSvc: permSvc, authSvc: authSvc}
}

type profileResponse struct {
	User   *auth.Claims `json:"user"`
	Roles  []string     `json:"roles"`
	Perms  []string     `json:"perms"`
	Menus  any          `json:"menus"`
	Routes any          `json:"routes"`
}

func (c *AuthController) Profile(ctx *gin.Context) {
	claimsAny, ok := ctx.Get("claims")
	if !ok {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	claims, ok := claimsAny.(*auth.Claims)
	if !ok || claims == nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}

	roles, err := c.permSvc.UserRoles(ctx.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}
	roleCodes := make([]string, 0, len(roles))
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
	}

	perms, err := c.permSvc.UserPermCodes(ctx.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}

	menus, err := c.permSvc.UserMenus(ctx.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}

	routes, err := c.permSvc.UserRoutes(ctx.Request.Context(), claims.UserID)
	if err != nil {
		resp.Fail(ctx, err)
		return
	}

	resp.OK(ctx, profileResponse{
		User:   claims,
		Roles:  roleCodes,
		Perms:  perms,
		Menus:  menus,
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
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	res, err := c.authSvc.Refresh(ctx.Request.Context(), req.RefreshToken)
	if err != nil {
		resp.Fail(ctx, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	resp.OK(ctx, res)
}

// Logout 注销当前 access token（从 Authorization 头取）。
func (c *AuthController) Logout(ctx *gin.Context) {
	raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(ctx.GetHeader("Authorization")), "Bearer "))
	if raw == "" {
		resp.Fail(ctx, errs.New(errs.CodeParamMissing, nil))
		return
	}
	if err := c.authSvc.Logout(ctx.Request.Context(), raw); err != nil {
		resp.Fail(ctx, err)
		return
	}
	resp.OK(ctx, nil)
}
