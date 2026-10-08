package service

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
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
	// 回滚开关（spec §4.2.6）：mode=jwt 时签发旧静态 JWT，走中间件 jwt 分支；
	// 不透明令牌只在 token 模式签发。permSvc 同步与模式无关，两个分支之前已执行。
	if s.cfg != nil && s.cfg.Auth.Mode == "jwt" {
		// 回滚模式：签发旧静态 JWT（中间件 jwt 分支按此校验）。
		tk, terr := auth.GenerateToken(u.ID, u.Username, tid, s.cfg)
		if terr != nil {
			return nil, nil, terr
		}
		return &LoginResult{AccessToken: tk}, u, nil
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
