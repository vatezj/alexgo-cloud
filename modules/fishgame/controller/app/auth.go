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

// Login godoc
//
//	@Summary		游戏端登录（微信 code 换会话）
//	@Description	body {"code":"..."} → {"session":"..."}；失败 401 {"error":"login_failed"}。免 X-Session（自鉴权入口，全裸 JSON 无 pkg/resp 信封）。
//	@Tags			fishgame-app
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object		true	"微信登录 code"	Example({"code": "wx_login_code"})
//	@Success		200		{object}	map[string]string
//	@Failure		400		{object}	map[string]string
//	@Failure		401		{object}	map[string]string
//	@Router			/v1/auth/login [post]
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
