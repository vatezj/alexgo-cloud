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
	Login   oauth2svc.UnifiedLogin
	Tokens  *oauth2svc.TokenService
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
	r.POST("/oauth2/authorize/login", p.authorizeLogin)
	r.POST("/oauth2/token", p.token)
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

// authorizeLogin 统一登录表单提交（spec §4.2 ②③；JSON body——裁决见任务头）。
// 校验顺序与 GET /authorize 一致：形状/白名单校验全部先于凭据，
// **前 6 步失败一律 400 且无 Location**（失效/篡改绝不 302，D15 补充语义）；
// 凭据经 Login.AuthorizeLogin（不签发 token）；成功才 Codes.Issue 并 302 回跳。
func (p *ProtocolController) authorizeLogin(c *gin.Context) {
	var req struct {
		ClientID            string `json:"client_id"`
		RedirectURI         string `json:"redirect_uri"`
		State               string `json:"state"`
		Scope               string `json:"scope"`
		ResponseType        string `json:"response_type"`
		Username            string `json:"username"`
		Password            string `json:"password"`
		AccountType         string `json:"account_type"`
		CaptchaUUID         string `json:"captcha_uuid"`
		CaptchaCode         string `json:"captcha_code"`
		CodeChallenge       string `json:"code_challenge"`
		CodeChallengeMethod string `json:"code_challenge_method"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "请求体必须是 JSON 对象")
		return
	}
	if req.ClientID == "" || req.RedirectURI == "" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "缺少 client_id/redirect_uri")
		return
	}
	ctx := c.Request.Context()
	client, err := p.deps.Clients.Get(ctx, req.ClientID)
	if err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_client", "未知或停用的 client")
		return
	}
	if !client.AllowsGrant(oauth2svc.GrantAuthorizationCode) {
		p.protocolErr(c, http.StatusBadRequest, "unauthorized_client", "client 未获授权码许可")
		return
	}
	if !client.AllowsRedirect(req.RedirectURI) {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "redirect_uri 不在白名单")
		return
	}
	if req.ResponseType != "" && req.ResponseType != "code" {
		p.protocolErr(c, http.StatusBadRequest, "unsupported_response_type", "仅支持 response_type=code")
		return
	}
	if req.CodeChallenge != "" && req.CodeChallengeMethod != "S256" {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "PKCE 仅支持 S256")
		return
	}
	ident, lerr := p.deps.Login.AuthorizeLogin(ctx, oauth2svc.LoginParams{
		AccountType: req.AccountType, // 空值由统一登录缺省 system（Task 8 checkAndRoute）
		Username:    req.Username,
		Password:    req.Password,
		ClientID:    req.ClientID,
		CaptchaUUID: req.CaptchaUUID,
		CaptchaCode: req.CaptchaCode,
		IP:          c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	})
	if lerr != nil {
		pe := oauth2svc.LoginProtoErr(lerr)
		p.protocolErr(c, pe.Status, pe.Code, pe.Description)
		return
	}
	code, ierr := p.deps.Codes.Issue(ctx, oauth2svc.IssueCodeParams{
		ClientID: req.ClientID, UserID: ident.UserID, Username: ident.Username,
		AccountType: ident.AccountType, TenantID: ident.TenantID,
		RedirectURI: req.RedirectURI, Scopes: req.Scope,
		CodeChallenge: req.CodeChallenge, CodeChallengeMethod: req.CodeChallengeMethod,
	})
	if ierr != nil {
		p.protocolErr(c, http.StatusInternalServerError, "server_error", "签发授权码失败")
		return
	}
	vals := url.Values{}
	vals.Set("code", code)
	if req.State != "" {
		vals.Set("state", req.State)
	}
	if req.Scope != "" {
		vals.Set("scope", req.Scope)
	}
	c.Redirect(http.StatusFound, req.RedirectURI+"?"+vals.Encode())
}

// token 四 grant 统一换 token（spec §5；JSON body——裁决见任务头）。
// 成功 200 信封 TokenResponse；失败把 TokenService 的 *ProtocolError 原样写出。
func (p *ProtocolController) token(c *gin.Context) {
	var req oauth2svc.TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		p.protocolErr(c, http.StatusBadRequest, "invalid_request", "请求体必须是 JSON 对象")
		return
	}
	req.IP = c.ClientIP()
	req.UserAgent = c.GetHeader("User-Agent")
	resp, perr := p.deps.Tokens.Exchange(c.Request.Context(), &req)
	if perr != nil {
		p.protocolErr(c, perr.Status, perr.Code, perr.Description)
		return
	}
	c.JSON(http.StatusOK, resp)
}
