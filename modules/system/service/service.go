package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	tenantSvc    TenantService // 账号额度检查（触额拒绝建号）
}

// NewService 构造服务；tenantSvc 由同模块 NewTenantService 解析（FX 自动注入）。
func NewService(
	repo repository.UserRepository,
	userRoleRepo repository.UserRoleRepository,
	roleRepo repository.RoleRepository,
	permSvc PermissionService,
	tenantSvc TenantService,
) UserService {
	return &userService{
		repo:         repo,
		userRoleRepo: userRoleRepo,
		roleRepo:     roleRepo,
		permSvc:      permSvc,
		tenantSvc:    tenantSvc,
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
	// 用户名禁止含 ':'：角色 sub 命名空间为 "{tid}:role:{code}"，用户 sub 为
	// "{tid}:{username}"。若用户名可含 ':'，建号 "role:admin" 会得到
	// "{tid}:role:admin"，与管理员角色 sub 撞车 → casbin g(x,x) 恒等 → 提权。
	if strings.Contains(username, ":") {
		return nil, fmt.Errorf("username/nickname must not contain ':'")
	}
	tid := tenant.TenantIDFromContext(ctx)

	if _, err := s.repo.GetByUsername(ctx, tid, username); err == nil {
		return nil, fmt.Errorf("username already exists")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	// 账号额度闸门（与 member 注册共用 TenantService.CheckAccountLimit），写库前拦截。
	if err := s.tenantSvc.CheckAccountLimit(ctx, tid); err != nil {
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
