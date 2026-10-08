package service

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/auth"
	"alexGo-cloud/pkg/config"
	"alexGo-cloud/pkg/tenant"
)

type AuthService interface {
	Login(ctx context.Context, username, password string) (string, *model.User, error)
}

type authService struct {
	cfg      *config.Config
	userRepo repository.UserRepository
	permSvc  PermissionService
}

func NewAuthService(cfg *config.Config, userRepo repository.UserRepository, permSvc PermissionService) AuthService {
	return &authService{cfg: cfg, userRepo: userRepo, permSvc: permSvc}
}

func (s *authService) Login(ctx context.Context, username, password string) (string, *model.User, error) {
	if username == "" || password == "" {
		return "", nil, fmt.Errorf("username or password is empty")
	}
	tid := tenant.TenantIDFromContext(ctx)
	u, err := s.userRepo.GetByUsername(ctx, tid, username)
	if err != nil {
		return "", nil, err
	}
	if u.Status != 1 {
		return "", nil, fmt.Errorf("user disabled")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, fmt.Errorf("invalid credentials")
	}
	if s.permSvc != nil {
		roles, _ := s.permSvc.UserRoles(ctx, u.ID)
		_ = s.permSvc.EnsureUserRolePolicy(ctx, u.Username, roles)
	}
	token, err := auth.GenerateToken(u.ID, u.Username, tid, s.cfg)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}
