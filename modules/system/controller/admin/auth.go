package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
)

type AuthController struct {
	permSvc service.PermissionService
}

func NewAuthController(permSvc service.PermissionService) *AuthController {
	return &AuthController{permSvc: permSvc}
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
