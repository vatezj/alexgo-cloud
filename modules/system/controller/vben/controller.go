// Package vben 提供 vben-admin 前端契约的薄壳端点（5 条），
// 把请求翻译到既有 service，不承载业务逻辑。
package vben

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	"alexGo-cloud/modules/system/service"
	"alexGo-cloud/pkg/auth"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
)

type Controller struct {
	unified oauth2svc.UnifiedLogin
	authSvc service.AuthService
	permSvc service.PermissionService
	userSvc service.UserService
}

func NewController(
	login oauth2svc.UnifiedLogin,
	authSvc service.AuthService,
	permSvc service.PermissionService,
	userSvc service.UserService,
) *Controller {
	return &Controller{
		unified: login,
		authSvc: authSvc,
		permSvc: permSvc,
		userSvc: userSvc,
	}
}

// Register 把 5 个 vben 端点挂到 server 的 /api 根组（全路径 /api/auth/login 等）。
func (ctrl *Controller) Register(r *gin.RouterGroup) {
	r.POST("/auth/login", ctrl.Login)
	r.POST("/auth/logout", ctrl.Logout)
	r.GET("/auth/codes", ctrl.Codes)
	r.GET("/user/info", ctrl.UserInfo)
	r.GET("/menu/all", ctrl.MenuAll)
}

// loginData vben 登录成功载荷（camelCase 字段与旧版逐字一致，前端读 accessToken）。
type loginData struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"`
}

// Login POST /api/auth/login —— vben 登录页契约（内部走统一登录；D13 审计在服务层恰一次）。
// 映射：captcha 拒绝 → 400（30005）；其余错误 → 401（20001 固定文案，防枚举）。
func (ctrl *Controller) Login(c *gin.Context) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		CaptchaUUID string `json:"captcha_uuid"`
		CaptchaCode string `json:"captcha_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, errs.New(errs.CodeParamInvalid, err))
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
			resp.Fail(c, errs.New(errs.CodeCaptchaInvalid, nil))
			return
		}
		resp.Fail(c, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	resp.OK(c, loginData{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
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
	resp.OK(c, nil)
}

// Codes GET /api/auth/codes —— 返回原始 perm 字符串数组（v-access 逐字匹配，含 `*` 形态）。
func (ctrl *Controller) Codes(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		resp.Fail(c, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	codes, err := ctrl.permSvc.UserPermCodes(c.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	if codes == nil {
		codes = []string{}
	}
	resp.OK(c, codes)
}

// userInfoData 对齐 vben BasicUserInfo + UserInfo：
// userId 必须字符串；desc/homePath/token 三个必填字段恒在。
type userInfoData struct {
	UserID   string   `json:"userId"`
	Username string   `json:"username"`
	RealName string   `json:"realName"`
	Avatar   string   `json:"avatar"`
	Roles    []string `json:"roles"`
	Desc     string   `json:"desc"`
	HomePath string   `json:"homePath"`
	Token    string   `json:"token"`
}

// UserInfo GET /api/user/info —— 字段形状见 userInfoData。
func (ctrl *Controller) UserInfo(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		resp.Fail(c, errs.New(errs.CodeUnauthorized, nil))
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
	resp.OK(c, userInfoData{
		UserID:   strconv.FormatUint(cl.UserID, 10),
		Username: cl.Username,
		RealName: realName,
		Avatar:   avatar,
		Roles:    roles,
		Desc:     "",
		HomePath: "",
		Token:    strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")),
	})
}

// MenuAll GET /api/menu/all —— vben backend 模式的完整路由树。
// 目录占位 LAYOUT 在此置空：vben convertRoutes 对空 component 直接跳过，
// generateAccessible 随后 delete 有 children 顶层路由的 component；
// 若输出 "LAYOUT" 会走 pageMap 查找失败分支（console.error + not-found 组件）。
func (ctrl *Controller) MenuAll(c *gin.Context) {
	cl, found := claimsOf(c)
	if !found {
		resp.Fail(c, errs.New(errs.CodeUnauthorized, nil))
		return
	}
	routes, err := ctrl.permSvc.UserRoutes(c.Request.Context(), cl.UserID)
	if err != nil {
		resp.Fail(c, err)
		return
	}
	if routes == nil {
		routes = []*service.VbenRoute{}
	}
	blankLayout(routes)
	resp.OK(c, routes)
}

// blankLayout 递归把 component=="LAYOUT" 置空（UserRoutes 每次请求新建树，原地改安全）。
func blankLayout(routes []*service.VbenRoute) {
	for _, r := range routes {
		if r == nil {
			continue
		}
		if r.Component == "LAYOUT" {
			r.Component = ""
		}
		blankLayout(r.Children)
	}
}
