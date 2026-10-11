package app

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
)

type AppController struct {
	unified oauth2svc.UnifiedLogin
}

func NewAppController(unified oauth2svc.UnifiedLogin) *AppController {
	return &AppController{unified: unified}
}

type loginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	CaptchaUUID string `json:"captcha_uuid"`
	CaptchaCode string `json:"captcha_code"`
}

// Login app 端登录（怪癖保留：除 captcha 与 bind 外一律 500——既有前端契约）。
// 审计归统一登录（D13），本层不再记账。
func (ctrl *AppController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, _, err := ctrl.unified.Login(c.Request.Context(), oauth2svc.LoginParams{
		AccountType: oauth2svc.AccountTypeSystem,
		Username:    req.Username,
		Password:    req.Password,
		ClientID:    "alexgo-admin",
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if err != nil {
		if errors.Is(err, oauth2svc.ErrCaptchaInvalid) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 响应字段与旧版逐字一致：前端读 res.token 不变。
	c.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}
