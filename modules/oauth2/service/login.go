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

// unifiedLogin 统一登录实现（Task 4 占位 stub；Task 8 删除方法体换真逻辑，
// 接口/哨兵/ProtocolError/UnifiedDeps 不动——见计划 Task 8 开头 Ruling）。
type unifiedLogin struct {
	deps UnifiedDeps
}

// NewUnifiedLogin Task 4 仅建构造与结构占位，保证编译与图装配；
// 真身逻辑（Login/AuthorizeLogin/IssueFor）归 Task 8。
func NewUnifiedLogin(deps UnifiedDeps) UnifiedLogin {
	return &unifiedLogin{deps: deps}
}

// Login 占位：Task 8 实现（类型校验→captcha→路由→IssueFor→恰一次审计）。
func (u *unifiedLogin) Login(context.Context, LoginParams) (*LoginResult, *AuthIdentity, error) {
	return nil, nil, ErrAuthenticatorMissing
}

// AuthorizeLogin 占位：Task 8 实现（不签发 token 的授权码流登录）。
func (u *unifiedLogin) AuthorizeLogin(context.Context, LoginParams) (*AuthIdentity, error) {
	return nil, ErrAuthenticatorMissing
}

// IssueFor 占位：Task 8 实现（按账号类型与模式签发）。
func (u *unifiedLogin) IssueFor(context.Context, *AuthIdentity, string) (*LoginResult, error) {
	return nil, ErrAuthenticatorMissing
}
