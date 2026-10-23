package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/fishgame/service"
)

// AuthController 游戏端登录（POST /api/v1/auth/login）。裸 JSON，不调 pkg/resp。
type AuthController struct {
	svc *service.AuthService
}

func NewAuthController(svc *service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

// Login body {"code":"..."} → {"session":"..."}；失败 401 {"error":"login_failed"}。
func (c *AuthController) Login(ctx *gin.Context) {
	var body struct {
		Code string `json:"code" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_body"})
		return
	}
	token, err := c.svc.Login(ctx.Request.Context(), body.Code)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "login_failed"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"session": token})
}
