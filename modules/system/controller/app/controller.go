package app

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

type AppController struct {
	authSvc  service.AuthService
	auditSvc service.AuditService
}

func NewAppController(authSvc service.AuthService, auditSvc service.AuditService) *AppController {
	return &AppController{authSvc: authSvc, auditSvc: auditSvc}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (ctrl *AppController) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, u, err := ctrl.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if ctrl.auditSvc != nil {
			_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), req.Username, 0, c.ClientIP(), c.GetHeader("User-Agent"), false, err.Error())
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ctrl.auditSvc != nil && u != nil {
		_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), u.Username, u.ID, c.ClientIP(), c.GetHeader("User-Agent"), true, "ok")
	}
	// 响应新增 refresh_token/expires_in 向后兼容：token 字段名与旧版一致（前端读 res.token 不变）。
	c.JSON(http.StatusOK, gin.H{
		"token": res.AccessToken, "refresh_token": res.RefreshToken, "expires_in": res.ExpiresIn,
	})
}
