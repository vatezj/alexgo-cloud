package service

import (
	"context"
	"errors"

	"go.uber.org/fx"

	"alexGo-cloud/modules/system/oauth2/model"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// RFC 6749 grant 类型（表 1）。
const (
	GrantAuthorizationCode = "authorization_code"
	GrantPassword          = "password"
	GrantClientCredentials = "client_credentials"
	GrantRefreshToken      = "refresh_token"
)

// LoginProtoErr 统一登录错误 → RFC 6749 协议错误（Task 11 控制器复用）。
// 映射矩阵是冻结契约（Review Focus：错误码不能随实现漂移）：
// ErrInvalidCredentials→400 invalid_grant「账号或密码错误」；
// ErrUserDisabled→400 invalid_grant「账号已停用」；ErrCaptchaInvalid→400 captcha_invalid；
// ErrIssuance|ErrAuthenticatorMissing→500 server_error「签发服务不可用」；
// ErrAccountTypeUnsupported→400 invalid_request；其余→500 server_error。
func LoginProtoErr(err error) *ProtocolError {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号或密码错误"}
	case errors.Is(err, ErrUserDisabled):
		return &ProtocolError{Status: 400, Code: "invalid_grant", Description: "账号已停用"}
	case errors.Is(err, ErrCaptchaInvalid):
		return &ProtocolError{Status: 400, Code: "captcha_invalid", Description: "验证码缺失或错误"}
	case errors.Is(err, ErrIssuance), errors.Is(err, ErrAuthenticatorMissing):
		return &ProtocolError{Status: 500, Code: "server_error", Description: "签发服务不可用"}
	case errors.Is(err, ErrAccountTypeUnsupported):
		return &ProtocolError{Status: 400, Code: "invalid_request", Description: "不支持的账号类型"}
	default:
		return &ProtocolError{Status: 500, Code: "server_error", Description: "内部错误"}
	}
}

// TokenRequest POST /oauth2/token 请求体（RFC 6749 §5.1 字段 + D12 captcha 两字段）。
// IP/UserAgent 由 controller 从请求上下文填充，不进 JSON。
type TokenRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Code         string `json:"code"`
	RedirectURI  string `json:"redirect_uri"`
	CodeVerifier string `json:"code_verifier"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	AccountType  string `json:"account_type"`
	RefreshToken string `json:"refresh_token"`
	CaptchaUUID  string `json:"captcha_uuid"`
	CaptchaCode  string `json:"captcha_code"`
	IP           string `json:"-"` // controller 填充
	UserAgent    string `json:"-"`
}

// TokenResponse RFC 6749 §5.1 成功信封。
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"` // 恒 "Bearer"
}

// TokenServiceDeps Fx 装配结构（Task 11 随控制器依赖一起接 Provide）。
type TokenServiceDeps struct {
	fx.In
	Cfg     *config.Config
	Clients *ClientService
	Codes   *AuthorizeCodeService
	Login   UnifiedLogin
	Issuer  token.Issuer
}

type TokenService struct {
	deps TokenServiceDeps
}

func NewTokenService(deps TokenServiceDeps) *TokenService {
	return &TokenService{deps: deps}
}

func toTokenResponse(r *LoginResult) *TokenResponse {
	return &TokenResponse{
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
		ExpiresIn:    r.ExpiresIn,
		TokenType:    "Bearer",
	}
}

// Exchange 四 grant 状态机。第一步恒为 client 认证（所有 grant 通用）：
// 失败 → 401 invalid_client（含公开客户端走机机、未知 client、停用、错 secret）。
func (s *TokenService) Exchange(ctx context.Context, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	client, err := s.deps.Clients.Authenticate(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, &ProtocolError{Status: 401, Code: "invalid_client", Description: "client 认证失败"}
	}
	switch req.GrantType {
	case GrantPassword:
		return s.passwordGrant(ctx, client, req)
	case GrantClientCredentials:
		return s.clientCredentialsGrant(ctx, client, req)
	case GrantRefreshToken:
		return s.refreshGrant(ctx, client, req)
	case GrantAuthorizationCode:
		return s.authCodeGrant(ctx, client, req)
	default:
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "不支持的 grant_type"}
	}
}

func (s *TokenService) passwordGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantPassword) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 password 许可"}
	}
	at := req.AccountType
	if at == "" {
		at = AccountTypeSystem
	}
	if !client.AllowsAccountType(at) {
		// Review Focus #3：账号类型白名单（防 admin client 换 member token）。
		return nil, &ProtocolError{Status: 400, Code: "unauthorized_client", Description: "client 不允许该账号类型"}
	}
	res, _, lerr := s.deps.Login.Login(ctx, LoginParams{
		AccountType: at, Username: req.Username, Password: req.Password,
		ClientID: req.ClientID, CaptchaUUID: req.CaptchaUUID, CaptchaCode: req.CaptchaCode,
		IP: req.IP, UserAgent: req.UserAgent,
	})
	if lerr != nil {
		return nil, LoginProtoErr(lerr)
	}
	return toTokenResponse(res), nil
}

func (s *TokenService) clientCredentialsGrant(ctx context.Context, client *model.Oauth2Client, _ *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantClientCredentials) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 client_credentials 许可"}
	}
	if client.ClientSecret == "" {
		// Review Focus #3：公开客户端禁机机凭证。
		return nil, &ProtocolError{Status: 401, Code: "invalid_client", Description: "公开客户端不支持机机凭证"}
	}
	ident := &AuthIdentity{Username: client.ClientID, AccountType: AccountTypeClient}
	res, ierr := s.deps.Login.IssueFor(ctx, ident, client.ClientID)
	if ierr != nil {
		return nil, LoginProtoErr(ierr)
	}
	return toTokenResponse(res), nil
}

func (s *TokenService) refreshGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantRefreshToken) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 refresh_token 许可"}
	}
	if req.RefreshToken == "" {
		// T6 缺口收口：空串会命中 client 行 refresh_token='' 并「刷新」出新 token，必须先拒。
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "刷新令牌无效"}
	}
	if s.deps.Cfg != nil && s.deps.Cfg.Auth.Mode == "jwt" {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "jwt 模式不支持 refresh"}
	}
	issued, rerr := s.deps.Issuer.Refresh(ctx, req.RefreshToken)
	if rerr != nil {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "刷新令牌无效"}
	}
	// D14：refresh 不做 client 绑定（refresh token 即凭证），只过了上面的 client 活跃+grant 白名单。
	return &TokenResponse{
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
		ExpiresIn:    issued.ExpiresIn,
		TokenType:    "Bearer",
	}, nil
}

func (s *TokenService) authCodeGrant(ctx context.Context, client *model.Oauth2Client, req *TokenRequest) (*TokenResponse, *ProtocolError) {
	if !client.AllowsGrant(GrantAuthorizationCode) {
		return nil, &ProtocolError{Status: 400, Code: "unsupported_grant_type", Description: "client 未获 authorization_code 许可"}
	}
	row, cerr := s.deps.Codes.Exchange(ctx, ExchangeParams{
		Code: req.Code, ClientID: req.ClientID,
		RedirectURI: req.RedirectURI, CodeVerifier: req.CodeVerifier,
	})
	if cerr != nil {
		return nil, &ProtocolError{Status: 400, Code: "invalid_grant", Description: "授权码无效"}
	}
	if !client.AllowsAccountType(row.AccountType) {
		return nil, &ProtocolError{Status: 400, Code: "unauthorized_client", Description: "client 不允许该账号类型"}
	}
	ident := &AuthIdentity{
		UserID: row.UserID, TenantID: row.OwnerTenantID,
		Username: row.Username, AccountType: row.AccountType,
	}
	res, ierr := s.deps.Login.IssueFor(ctx, ident, row.ClientID)
	if ierr != nil {
		return nil, LoginProtoErr(ierr)
	}
	return toTokenResponse(res), nil
}
