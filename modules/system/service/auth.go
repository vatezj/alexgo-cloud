package service

import (
	"context"
	"fmt"

	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/token"
)

// LoginResult 是双端登录的统一响应（controller 直接序列化）。
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthService 登录已迁统一登录（Task 12：vben+app 是仅有的 Login 调用方，
// member 走 MemberService），故缩为刷新/登出两方法。
type AuthService interface {
	Refresh(ctx context.Context, refreshToken string) (*LoginResult, error)
	Logout(ctx context.Context, accessToken string) error
}

type authService struct {
	cfg      *config.Config
	userRepo repository.UserRepository
	permSvc  PermissionService
	issuer   token.Issuer
}

func NewAuthService(cfg *config.Config, userRepo repository.UserRepository, permSvc PermissionService, issuer token.Issuer) AuthService {
	return &authService{cfg: cfg, userRepo: userRepo, permSvc: permSvc, issuer: issuer}
}

func toResult(i *token.Issued) *LoginResult {
	return &LoginResult{AccessToken: i.AccessToken, RefreshToken: i.RefreshToken, ExpiresIn: i.ExpiresIn}
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	// 回滚模式：静态 JWT 无刷新轮换语义，直接拒绝（前端应重新登录）。
	if s.cfg != nil && s.cfg.Auth.Mode == "jwt" {
		return nil, fmt.Errorf("refresh not supported in jwt mode")
	}
	issued, err := s.issuer.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

func (s *authService) Logout(ctx context.Context, accessToken string) error {
	// 已知限制（回滚模式）：静态 JWT 无状态、无法吊销，logout 视为 no-op——
	// 想立即踢人只能回到 token 模式（或改 secret 让存量 JWT 全体失效）。
	if s.cfg != nil && s.cfg.Auth.Mode == "jwt" {
		return nil
	}
	return s.issuer.Revoke(ctx, accessToken)
}
