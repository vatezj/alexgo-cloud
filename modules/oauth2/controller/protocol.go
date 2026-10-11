package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
)

// ProtocolController 协议端点（RFC 6749 信封）。
type ProtocolController struct {
	captcha *oauth2svc.CaptchaService
}

// NewProtocolController 初版只接 captcha；Task 11 扩展为 ProtocolDeps。
func NewProtocolController(captcha *oauth2svc.CaptchaService) *ProtocolController {
	return &ProtocolController{captcha: captcha}
}

// Register 挂 /oauth2/*（module 传入的分组已带 /api 前缀）。
func (p *ProtocolController) Register(r gin.IRouter) {
	r.GET("/oauth2/captcha", p.captchaImage)
}

func (p *ProtocolController) captchaImage(c *gin.Context) {
	ch, err := p.captcha.NewChallenge(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "captcha generate failed", "error_description": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ch)
}
