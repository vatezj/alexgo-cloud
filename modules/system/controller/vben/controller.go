// Package vben 提供 vben-admin 前端契约的薄壳端点（5 条），
// 把请求翻译到既有 service，不承载业务逻辑。
package vben

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
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
	r.POST("/auth/logout", ctrl.Logout)
	r.GET("/auth/codes", ctrl.Codes)
	r.GET("/user/info", ctrl.UserInfo)
	// /menu/all 由 Task 3 补齐
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

// claimsOf 取中间件注入的 claims（生产路径由 auth 中间件保证；handler 内再判一次作纵深防御）。
func claimsOf(c *gin.Context) (*auth.Claims, bool) {
	v, exists := c.Get("claims")
	if !exists {
		return nil, false
	}
	cl, ok := v.(*auth.Claims)
	return cl, ok && cl != nil
}

// Logout POST /api/auth/logout —— 恒 200：无 token/无效 token 一律 no-op，
// vben 退出是前端状态清理，不能因后端失败把用户卡在登出流程里。
func (ctrl *Controller) Logout(c *gin.Context) {
	if raw := c.GetHeader("Authorization"); raw != "" {
		tok := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "Bearer "))
		if tok != "" {
			_ = ctrl.authSvc.Logout(c.Request.Context(), tok)
		}
	}
	ok(c, gin.H{})
}

// Codes GET /api/auth/codes —— 返回原始 perm 字符串数组（v-access 逐字匹配，含 `*` 形态）。
func (ctrl *Controller) Codes(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	codes, err := ctrl.permSvc.UserPermCodes(c.Request.Context(), cl.UserID)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if codes == nil {
		codes = []string{}
	}
	ok(c, codes)
}

// UserInfo GET /api/user/info —— 对齐 vben BasicUserInfo + UserInfo：
// userId 必须字符串；desc/homePath/token 三个必填字段恒在。
func (ctrl *Controller) UserInfo(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		fail(c, http.StatusUnauthorized, "unauthorized")
		return
	}
	realName := cl.Username
	avatar := ""
	if u, err := ctrl.userSvc.GetUserByID(c.Request.Context(), cl.UserID); err == nil && u != nil {
		if u.Nickname != "" {
			realName = u.Nickname
		}
		avatar = u.Avatar
	}
	roles := []string{}
	if rs, err := ctrl.permSvc.UserRoles(c.Request.Context(), cl.UserID); err == nil {
		for _, r := range rs {
			if r != nil && r.Code != "" {
				roles = append(roles, r.Code)
			}
		}
	}
	ok(c, gin.H{
		"userId":   strconv.FormatUint(cl.UserID, 10),
		"username": cl.Username,
		"realName": realName,
		"avatar":   avatar,
		"roles":    roles,
		"desc":     "",
		"homePath": "",
		"token":    strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")),
	})
}
