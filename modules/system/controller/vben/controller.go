// Package vben 提供 vben-admin 前端契约的薄壳端点（5 条），
// 把请求翻译到既有 service，不承载业务逻辑。
package vben

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
)

var errInvalidCredentials = errors.New("invalid username or password")

type Controller struct {
	authSvc  service.AuthService
	auditSvc service.AuditService
	permSvc  service.PermissionService
	userSvc  service.UserService
}

func NewController(
	authSvc service.AuthService,
	auditSvc service.AuditService,
	permSvc service.PermissionService,
	userSvc service.UserService,
) *Controller {
	return &Controller{
		authSvc:  authSvc,
		auditSvc: auditSvc,
		permSvc:  permSvc,
		userSvc:  userSvc,
	}
}

// Register 把 5 个 vben 端点挂到 server 的 /api 根组（全路径 /api/auth/login 等）。
func (ctrl *Controller) Register(r *gin.RouterGroup) {
	r.POST("/auth/login", ctrl.Login)
}

// ok vben 成功信封。
func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data, "error": nil, "message": "ok"})
}

// fail vben 失败信封：HTTP 状态码与 code 同值，error/message 同文案。
func fail(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "data": nil, "error": msg, "message": msg})
}

// Login POST /api/auth/login —— vben 登录页契约。
// 失败：401 + 信封（errorMessage 拦截器会把 error 文案弹给用户）。
func (ctrl *Controller) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return
	}
	ip, ua := c.ClientIP(), c.GetHeader("User-Agent")
	res, u, err := ctrl.authSvc.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), req.Username, 0, ip, ua, false, err.Error())
		fail(c, http.StatusUnauthorized, errInvalidCredentials.Error())
		return
	}
	var userID uint64
	if u != nil {
		userID = u.ID
	}
	_ = ctrl.auditSvc.RecordLogin(c.Request.Context(), req.Username, userID, ip, ua, true, "ok")
	ok(c, gin.H{
		"accessToken":  res.AccessToken,
		"refreshToken": res.RefreshToken,
		"expiresIn":    res.ExpiresIn,
	})
}
