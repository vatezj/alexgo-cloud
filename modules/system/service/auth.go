package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/logger"
	"alexGo-cloud/pkg/tenant"
	"alexGo-cloud/pkg/token"
)

// LoginResult 是双端登录的统一响应（controller 直接序列化）。
type LoginResult struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthService interface {
	Login(ctx context.Context, username, password string) (*LoginResult, *model.User, error)
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

func (s *authService) clientID() string { return "alexgo-admin" }

func toResult(i *token.Issued) *LoginResult {
	return &LoginResult{AccessToken: i.AccessToken, RefreshToken: i.RefreshToken, ExpiresIn: i.ExpiresIn}
}

func (s *authService) Login(ctx context.Context, username, password string) (*LoginResult, *model.User, error) {
	if username == "" || password == "" {
		return nil, nil, fmt.Errorf("username or password is empty")
	}
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.userRepo.GetByUsername(ctx, tid, username)
	if err != nil {
		return nil, nil, err
	}
	if u.Status != 1 {
		return nil, nil, fmt.Errorf("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("invalid credentials")
	}
	if s.permSvc != nil {
		roles, rerr := s.permSvc.UserRoles(ctx, u.ID)
		if rerr != nil && logger.Log != nil {
			logger.Log.Warn("load user roles failed", zap.Error(rerr))
		}
		if perr := s.permSvc.EnsureUserRolePolicy(ctx, u.Username, roles); perr != nil && logger.Log != nil {
			logger.Log.Warn("ensure user role policy failed", zap.Error(perr))
		}
	}
	// 密码与状态校验都通过后才走到这里：Issue 是登录成功路径唯一的签发点（失败路径绝不签发）。
	issued, err := s.issuer.Issue(ctx, token.IssueParams{
		UserID: u.ID, UserType: token.UserTypeAdmin, TenantID: tid, ClientID: s.clientID(),
	})
	if err != nil {
		return nil, nil, err
	}
	return toResult(issued), u, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*LoginResult, error) {
	issued, err := s.issuer.Refresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return toResult(issued), nil
}

func (s *authService) Logout(ctx context.Context, accessToken string) error {
	return s.issuer.Revoke(ctx, accessToken)
}
