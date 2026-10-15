package service

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/fx"

	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// 本文件是统一登录的**冻结契约**（Task 5/8/10/11/12 逐字消费）：
// 账号类型常量、LoginParams/LoginResult/AuthIdentity、五哨兵、ProtocolError、
// UnifiedLogin 与四个窄接口、UnifiedDeps 装配结构。签名一个都不能改。

// 账号类型（三端 SSO 的路由依据，spec §2）。
const (
	AccountTypeSystem = "system"
	AccountTypeMember = "member"
	AccountTypeClient = "client"
)

// LoginParams 统一登录入参（controller 层解析后透传）。
type LoginParams struct {
	AccountType string
	Username    string
	Password    string
	ClientID    string
	CaptchaUUID string
	CaptchaCode string
	IP          string
	UserAgent   string
}

// LoginResult 令牌结果（JSON 字段对齐旧 /auth/login 响应）。
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthIdentity 凭据校验产物：路由与签发、审计共用的身份快照。
type AuthIdentity struct {
	UserID      uint64
	TenantID    uint64
	Username    string
	AccountType string
}

// UnifiedLogin 统一登录门面（Task 8 实现真身）。
type UnifiedLogin interface {
	// Login 完整登录：类型校验→captcha→路由→IssueFor→恰一次审计，返回 token。
	Login(ctx context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error)
	// AuthorizeLogin 授权码流登录：类型校验→captcha→路由→恰一次审计，
	// **不签发 token**（防止 authorize 侧产生无人认领的孤儿 token）；
	// p.ClientID 在此方法中忽略（client 校验归 controller）。Task 9/11 消费。
	AuthorizeLogin(ctx context.Context, p LoginParams) (*AuthIdentity, error)
	IssueFor(ctx context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error)
}

// UserAuthenticator system 账号凭据校验（实现归 system 模块）。
type UserAuthenticator interface {
	Authenticate(ctx context.Context, username, password string) (*AuthIdentity, error)
}

// MemberAuthenticator member 账号凭据校验（实现归 member 模块，Task 5）。
type MemberAuthenticator interface {
	Authenticate(ctx context.Context, mobile, password, ip string) (*AuthIdentity, error)
}

// JWTGenerator jwt 回滚模式（mode=jwt）的旧静态 JWT 签发（D2，实现归 system）。
type JWTGenerator interface {
	Issue(ctx context.Context, userID uint64, username string, tenantID uint64) (string, error)
}

// LoginAuditor 登录审计窄接口（实现为 system 的 AuditService.RecordLogin，D13）。
type LoginAuditor interface {
	RecordLogin(ctx context.Context, username string, userID uint64, ip, ua string, success bool, msg string) error
}

// ProtocolError RFC 6749 错误（Error() = "%d %s: %s"）。
type ProtocolError struct {
	Status      int
	Code        string
	Description string
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Description)
}

// 五哨兵：错误映射矩阵按 errors.Is 区分（controller 映射到协议错误码）。
var (
	ErrInvalidCredentials     = errors.New("invalid credentials")
	ErrAccountTypeUnsupported = errors.New("account type unsupported")
	ErrAuthenticatorMissing   = errors.New("authenticator missing")
	ErrUserDisabled           = errors.New("user disabled")
	ErrIssuance               = errors.New("token issuance failed")
)

// UnifiedDeps Fx 装配结构：必填三项（Cfg/Issuer/Captcha），其余窄接口 optional
// （member 进程无 system 实现也能装配，D6/D7）。
type UnifiedDeps struct {
	fx.In
	Cfg        *config.Config
	Issuer     token.Issuer
	Captcha    *CaptchaService
	UserAuth   UserAuthenticator   `optional:"true"`
	MemberAuth MemberAuthenticator `optional:"true"`
	Auditor    LoginAuditor        `optional:"true"`
	JWTGen     JWTGenerator        `optional:"true"`
}

// === Task 8 真身（覆写 Task 4 stub；接口/哨兵/ProtocolError/UnifiedDeps 不动）===

type unifiedLogin struct {
	cfg        *config.Config
	issuer     token.Issuer
	captcha    *CaptchaService
	userAuth   UserAuthenticator
	memberAuth MemberAuthenticator
	auditor    LoginAuditor
	jwtGen     JWTGenerator
}

func NewUnifiedLogin(deps UnifiedDeps) UnifiedLogin {
	return &unifiedLogin{
		cfg:        deps.Cfg,
		issuer:     deps.Issuer,
		captcha:    deps.Captcha,
		userAuth:   deps.UserAuth,
		memberAuth: deps.MemberAuth,
		auditor:    deps.Auditor,
		jwtGen:     deps.JWTGen,
	}
}

// checkAndRoute 类型校验（先于 captcha 与审计）→ captcha → 按类型路由凭据校验。
// 不签发、不记审计——两者归 Login/AuthorizeLogin（每恰一次）。
func (u *unifiedLogin) checkAndRoute(ctx context.Context, p LoginParams) (*AuthIdentity, error) {
	at := p.AccountType
	if at == "" {
		at = AccountTypeSystem // 缺省 system（authorize/login 请求体 account_type 可选）
	}
	if at != AccountTypeSystem && at != AccountTypeMember {
		return nil, ErrAccountTypeUnsupported
	}
	if u.captcha != nil {
		if err := u.captcha.Verify(ctx, p.CaptchaUUID, p.CaptchaCode); err != nil {
			return nil, err
		}
	}
	switch at {
	case AccountTypeSystem:
		if u.userAuth == nil {
			return nil, ErrAuthenticatorMissing
		}
		return u.userAuth.Authenticate(ctx, p.Username, p.Password)
	default: // member
		if u.memberAuth == nil {
			return nil, ErrAuthenticatorMissing
		}
		return u.memberAuth.Authenticate(ctx, p.Username, p.Password, p.IP)
	}
}

// audit 恰一次记账（D13：msg 前缀 "oauth2: "；auditor nil 安全——图未接线时静默跳过）。
func (u *unifiedLogin) audit(ctx context.Context, p LoginParams, ident *AuthIdentity, success bool, cause error) {
	if u.auditor == nil {
		return
	}
	username, userID := p.Username, uint64(0)
	if ident != nil {
		username, userID = ident.Username, ident.UserID
	}
	msg := "oauth2: ok"
	if !success {
		msg = "oauth2: " + cause.Error()
	}
	_ = u.auditor.RecordLogin(ctx, username, userID, p.IP, p.UserAgent, success, msg)
}

func (u *unifiedLogin) Login(ctx context.Context, p LoginParams) (*LoginResult, *AuthIdentity, error) {
	ident, err := u.checkAndRoute(ctx, p)
	if err != nil {
		// ErrAccountTypeUnsupported 是请求形状错误，先于审计（无审计）；
		// 其余失败一律 RecordLogin(success=false)（spec §6）。
		if !errors.Is(err, ErrAccountTypeUnsupported) {
			u.audit(ctx, p, nil, false, err)
		}
		return nil, nil, err
	}
	res, ierr := u.IssueFor(ctx, ident, p.ClientID)
	if ierr != nil {
		u.audit(ctx, p, ident, false, ierr)
		return nil, nil, ierr
	}
	u.audit(ctx, p, ident, true, nil)
	return res, ident, nil
}

func (u *unifiedLogin) AuthorizeLogin(ctx context.Context, p LoginParams) (*AuthIdentity, error) {
	ident, err := u.checkAndRoute(ctx, p)
	if err != nil {
		if !errors.Is(err, ErrAccountTypeUnsupported) {
			u.audit(ctx, p, nil, false, err)
		}
		return nil, err
	}
	u.audit(ctx, p, ident, true, nil)
	return ident, nil
}

// IssueFor 按账号类型与模式签发（D2/D9 冻结语义）：
//   - client：恒不透明（jwt 模式也如此），refresh 强制空；
//   - system + mode=jwt：JWTGenerator，无 refresh、expires 0；
//   - system/member 其余：token.Issuer 按类型签不透明令牌。
//
// 任何签发失败 wrap ErrIssuance（协议端点 500 server_error 的映射依据）。
func (u *unifiedLogin) IssueFor(ctx context.Context, ident *AuthIdentity, clientID string) (*LoginResult, error) {
	if ident == nil {
		return nil, fmt.Errorf("%w: nil identity", ErrIssuance)
	}
	if u.cfg != nil && u.cfg.Auth.Mode == "jwt" && ident.AccountType == AccountTypeSystem {
		if u.jwtGen == nil {
			return nil, fmt.Errorf("%w: jwt generator not wired", ErrIssuance)
		}
		tk, err := u.jwtGen.Issue(ctx, ident.UserID, ident.Username, ident.TenantID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrIssuance, err)
		}
		return &LoginResult{AccessToken: tk}, nil
	}
	if u.issuer == nil {
		return nil, fmt.Errorf("%w: issuer not wired", ErrIssuance)
	}
	ip := token.IssueParams{
		UserID:   ident.UserID,
		TenantID: ident.TenantID,
		ClientID: clientID,
	}
	switch ident.AccountType {
	case AccountTypeSystem:
		ip.UserType = token.UserTypeAdmin
	case AccountTypeMember:
		ip.UserType = token.UserTypeMember
	case AccountTypeClient:
		ip.UserType = token.UserTypeClient
		ip.UserID, ip.TenantID = 0, 0
	default:
		return nil, fmt.Errorf("%w: unknown account type %q", ErrIssuance, ident.AccountType)
	}
	issued, err := u.issuer.Issue(ctx, ip)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIssuance, err)
	}
	res := &LoginResult{
		AccessToken:  issued.AccessToken,
		RefreshToken: issued.RefreshToken,
		ExpiresIn:    issued.ExpiresIn,
	}
	if ident.AccountType == AccountTypeClient {
		res.RefreshToken = "" // 机机无 refresh（D9），即便底层返回也强制清空
	}
	return res, nil
}
