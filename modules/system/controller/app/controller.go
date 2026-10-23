package app

import (
	"errors"

	"github.com/gin-gonic/gin"

	oauth2svc "alexGo-cloud/modules/system/oauth2/service"
	errs "alexGo-cloud/pkg/errors"
	"alexGo-cloud/pkg/resp"
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

// Login godoc
//
//	@Summary		app 端登录（用户名密码 + 图形验证码）
//	@Description	免鉴权（自鉴权入口）；统一 pkg/resp 信封。验证码错 → CodeCaptchaInvalid；其余失败透传 → 50000（怪癖保留：除 captcha 外一律 HTTP 500，既有前端契约）。data={token,refresh_token,expires_in}。
//	@Tags			system-app
//	@Accept			json
//	@Produce		json
//	@Param			body	body	object	true	"登录参数"	Example({"username": "admin", "password": "secret", "captcha_uuid": "uuid-x", "captcha_code": "abcd"})
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/app/system/auth/login [post]
func (ctrl *AppController) Login(c *gin.Context) {
	var req loginRequest
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
		// 怪癖保留：凭据错等裸错误透传 → 50000（HTTP 500 与旧契约一致，原始错误只进日志）。
		resp.Fail(c, err)
		return
	}
	// 响应字段与旧版逐字一致：LoginResult json 标签即 token/refresh_token/expires_in，
	// 前端读 res.token 不变（拦截器解 data 后拿到同一形状）。
	resp.OK(c, res)
}
