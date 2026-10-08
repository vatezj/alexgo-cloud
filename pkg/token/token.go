// Package token 实现 yudao 式 OAuth2 不透明令牌：随机串落库、DB 为权威，
// 支持签发/校验/刷新（轮换）/注销/踢人。单体模式本地直调本包；
// 双服务模式 member-server 经 gRPC 委托 system-server 调用本包（token.Issuer 接口是切换点）。
package token

import "context"

type UserType int8

const (
	UserTypeAdmin  UserType = 1
	UserTypeMember UserType = 2
)

type Claims struct {
	UserID   uint64
	Username string
	UserType UserType
	TenantID uint64
	DeptID   uint64 // 仅管理员有意义，会员为 0
}

type IssueParams struct {
	UserID   uint64
	UserType UserType
	TenantID uint64
	ClientID string
}

type Issued struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64 // 秒
}

type Issuer interface {
	Issue(ctx context.Context, p IssueParams) (*Issued, error)
	Refresh(ctx context.Context, refreshToken string) (*Issued, error)
	Revoke(ctx context.Context, accessToken string) error
	RevokeAll(ctx context.Context, userType UserType, userID uint64) error
}

type Validator interface {
	Validate(ctx context.Context, accessToken string) (*Claims, error)
}
