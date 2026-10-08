package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
	"alexGo-cloud/pkg/tenant"
)

type UserService interface {
	ListUsers(ctx context.Context) ([]*model.User, error)
	GetUserByID(ctx context.Context, id uint64) (*model.User, error)
	CreateUser(ctx context.Context, username, nickname, password string) (*model.User, error)
	UpdateUser(ctx context.Context, id uint64, nickname string, status int) error
	ResetPassword(ctx context.Context, id uint64, newPassword string) error
	SetRoles(ctx context.Context, id uint64, roleIDs []uint64) error
}

type userService struct {
	repo         repository.UserRepository
	userRoleRepo repository.UserRoleRepository
	roleRepo     repository.RoleRepository
	permSvc      PermissionService
}

func NewService(
	repo repository.UserRepository,
	userRoleRepo repository.UserRoleRepository,
	roleRepo repository.RoleRepository,
	permSvc PermissionService,
) UserService {
	return &userService{
		repo:         repo,
		userRoleRepo: userRoleRepo,
		roleRepo:     roleRepo,
		permSvc:      permSvc,
	}
}

func (s *userService) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.repo.List(ctx, tenant.TenantIDFromContext(ctx))
}

func (s *userService) GetUserByID(ctx context.Context, id uint64) (*model.User, error) {
	return s.repo.GetByID(ctx, tenant.TenantIDFromContext(ctx), id)
}

func (s *userService) CreateUser(ctx context.Context, username, nickname, password string) (*model.User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username or password empty")
	}
	tid := tenant.TenantIDFromContext(ctx)

	if _, err := s.repo.GetByUsername(ctx, tid, username); err == nil {
		return nil, fmt.Errorf("username already exists")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	u := &model.User{
		Username:     username,
		Nickname:     nickname,
		PasswordHash: string(hash),
		Status:       1,
		TenantID:     tid,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *userService) UpdateUser(ctx context.Context, id uint64, nickname string, status int) error {
	u, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	if nickname != "" {
		u.Nickname = nickname
	}
	if status != 0 {
		u.Status = status
	}
	u.UpdatedAt = time.Now()
	return s.repo.Update(ctx, u)
}

func (s *userService) ResetPassword(ctx context.Context, id uint64, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("password empty")
	}
	u, err := s.GetUserByID(ctx, id)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	u.UpdatedAt = time.Now()
	return s.repo.Update(ctx, u)
}

func (s *userService) SetRoles(ctx context.Context, id uint64, roleIDs []uint64) error {
	tid := tenant.TenantIDFromContext(ctx)
	if err := s.userRoleRepo.SetUserRoles(ctx, tid, id, roleIDs); err != nil {
		return err
	}
	if s.permSvc == nil {
		return nil
	}
	u, err := s.repo.GetByID(ctx, tid, id)
	if err != nil {
		return err
	}
	roles, _ := s.permSvc.UserRoles(ctx, u.ID)
	return s.permSvc.EnsureUserRolePolicy(ctx, u.Username, roles)
}
