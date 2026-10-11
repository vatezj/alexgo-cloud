package controller

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/pkg/config"
)

// ProtocolDeps 协议控制器依赖（Task 9 起 fx.In 结构体 ctor——
// Task 11 加 Login/Tokens、Task 13 加 Validator，ctor 签名不再变）。
type ProtocolDeps struct {
	fx.In
	Captcha *oauth2svc.CaptchaService
	Codes   *oauth2svc.AuthorizeCodeService
	Clients *oauth2svc.ClientService
	Cfg     *config.Config
}

// ProtocolController 协议端点（RFC 6749 信封 {"error","error_description"}，
// 与 admin 的 {error}/{data,total} 信封区分——spec §6）。
type ProtocolController struct {
	deps ProtocolDeps
}

func NewProtocolController(deps ProtocolDeps) *ProtocolController {
	return &ProtocolController{deps: deps}
}

// Register 挂 /oauth2/*（module 传入的分组已带 /api 前缀）。
func (p *ProtocolController) Register(r gin.IRouter) {
	r.GET("/oauth2/captcha", p.captchaImage)
	r.GET("/oauth2/authorize", p.authorize)
}

// protocolErr 写 RFC 6749 错误信封；不设 Location（400 绝不 302 的结构保证）。
func (p *ProtocolController) protocolErr(c *gin.Context, status int, code, desc string) {
	c.JSON(status, gin.H{"error": code, "error_description": desc})
}

func (p *ProtocolController) captchaImage(c *gin.Context) {
	ch, err := p.deps.Captcha.NewChallenge(c.Request.Context())
	if err != nil {
		p.protocolErr(c, http.StatusInternalServerError, "server_error", "验证码生成失败")
		return
	}
	c.JSON(http.StatusOK, ch)
}

// authorize 授权码入口（spec §4.2 ①）：
// 缺参 → 400 invalid_request；Get 失败 → 400 invalid_client；
// grant 白名单 → 400 unauthorized_client；redirect 精确匹配失败 → 400 invalid_request；
// response_type != code → 400 unsupported_response_type；
// challenge 非空且 method != S256 → 400 invalid_request。
// **以上全部无 Location（失效/篡改一律 400 绝不 302，D15 补充语义）。**
// 全部通过 → 302 cfg.Auth.LoginPage?client_id&redirect_uri&state&code_challenge&...
func (p *ProtocolController) authorize(c *gin.Context) {
	q := c.Request.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	if clientID == "" || redirectURI == "" || responseType == "" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "缺少 client_id/redirect_uri/response_type")
		return
	}
	ctx := c.Request.Context()
	client, err := p.deps.Clients.Get(ctx, clientID)
	if err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_client", "未知或停用的 client")
		return
	}
	if !client.AllowsGrant("authorization_code") {
		p.protocolErr(c, http.StatusBadRequest, "unauthorized_client", "client 未获授权码许可")
		return
	}
	if !client.AllowsRedirect(redirectURI) {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "redirect_uri 不在白名单")
		return
	}
	if responseType != "code" {
		p.protocolErr(c, http.StatusBadRequest, "unsupported_response_type", "仅支持 response_type=code")
		return
	}
	challenge := q.Get("code_challenge")
	method := q.Get("code_challenge_method")
	if challenge != "" && method != "S256" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "PKCE 仅支持 S256")
		return
	}
	loginPage := p.deps.Cfg.Auth.LoginPage
	if loginPage == "" {
		loginPage = "/auth/login"
	}
	vals := url.Values{}
	vals.Set("client_id", clientID)
	vals.Set("redirect_uri", redirectURI)
	vals.Set("response_type", responseType)
	for _, k := range []string{"state", "scope", "code_challenge", "code_challenge_method"} {
		if v := q.Get(k); v != "" {
			vals.Set(k, v)
		}
	}
	c.Redirect(http.StatusFound, loginPage+"?"+vals.Encode())
}
