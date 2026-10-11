package service

import (
	"context"

	"golang.org/x/crypto/bcrypt"

	oauth2svc "alexGo-cloud/modules/oauth2/service"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/tenant"
)

// NewUserAuthenticator system 账号凭据校验（users 表零改动，spec §2 边界1）。
func NewUserAuthenticator(userRepo repository.UserRepository, permSvc PermissionService) oauth2svc.UserAuthenticator {
	return &userAuthenticator{repo: userRepo, perm: permSvc}
}

type userAuthenticator struct {
	repo repository.UserRepository
	perm PermissionService
}

func (a *userAuthenticator) Authenticate(ctx context.Context, username, password string) (*oauth2svc.AuthIdentity, error) {
	tid := tenant.TenantIDFromContext(ctx)
	u, err := a.repo.GetByUsername(ctx, tid, username)
	if err != nil || u == nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	if u.Status != 1 {
		return nil, oauth2svc.ErrUserDisabled
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return nil, oauth2svc.ErrInvalidCredentials
	}
	// 登录成功补挂默认角色策略（对齐旧 AuthService.Login 行为：
	// 先取角色再 Ensure，失败仅告警不阻断——与 auth.go 64-72 行同构）。
	if a.perm != nil {
		if roles, rerr := a.perm.UserRoles(ctx, u.ID); rerr == nil {
			_ = a.perm.EnsureUserRolePolicy(ctx, u.Username, roles)
		}
	}
	return &oauth2svc.AuthIdentity{
		UserID:      u.ID,
		TenantID:    u.TenantID,
		Username:    u.Username,
		AccountType: oauth2svc.AccountTypeSystem,
	}, nil
}

// NewJWTGenerator jwt 回滚模式（mode=jwt）用（D2）。
func NewJWTGenerator(cfg *config.Config) oauth2svc.JWTGenerator {
	return &jwtGenerator{cfg: cfg}
}

type jwtGenerator struct{ cfg *config.Config }

func (g *jwtGenerator) Issue(ctx context.Context, userID uint64, username string, tenantID uint64) (string, error) {
	return auth.GenerateToken(userID, username, tenantID, g.cfg)
}
