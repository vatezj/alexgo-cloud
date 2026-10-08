package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alexGo-cloud/modules/system/model"
	"alexGo-cloud/modules/system/repository"
)

// TenantService 租户管理 + 账号额度检查（system 建用户与 member 注册共用 CheckAccountLimit）。
type TenantService interface {
	List(ctx context.Context) ([]*model.Tenant, error)
	Create(ctx context.Context, name, domain string, accountLimit int) (*model.Tenant, error)
	Update(ctx context.Context, id uint64, name, domain string, status, accountLimit int) error
	Delete(ctx context.Context, id uint64) error
	// CheckAccountLimit：account_limit>0 时校验租户账号数未超限（system 建用户与 member 注册共用）。
	CheckAccountLimit(ctx context.Context, tenantID uint64) error
}

type tenantService struct {
	repo repository.TenantRepository
}

// NewTenantService 构造服务（仅依赖租户仓储，无跨模块依赖）。
func NewTenantService(repo repository.TenantRepository) TenantService {
	return &tenantService{repo: repo}
}

func (s *tenantService) List(ctx context.Context) ([]*model.Tenant, error) {
	return s.repo.List(ctx)
}

func (s *tenantService) Create(ctx context.Context, name, domain string, accountLimit int) (*model.Tenant, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("tenant name required")
	}
	now := time.Now()
	t := &model.Tenant{
		Name: name, Domain: domain, Status: 1,
		AccountLimit: accountLimit, // 默认 -1 不限时由调用方传
		CreatedAt:    now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *tenantService) Update(ctx context.Context, id uint64, name, domain string, status, accountLimit int) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	t.Name, t.Domain, t.Status, t.AccountLimit = name, domain, status, accountLimit
	t.UpdatedAt = time.Now()
	return s.repo.Update(ctx, t)
}

func (s *tenantService) Delete(ctx context.Context, id uint64) error {
	return s.repo.Delete(ctx, id)
}

// CheckAccountLimit 账号额度闸门：tenantID=0（平台租户）不限；
// 租户不存在或触额返回 error（调用方原样透传给客户端）。
func (s *tenantService) CheckAccountLimit(ctx context.Context, tenantID uint64) error {
	if tenantID == 0 {
		return nil // 平台租户不限
	}
	t, err := s.repo.GetByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}
	if t.AccountLimit <= 0 {
		return nil
	}
	n, err := s.repo.CountAccounts(ctx, tenantID)
	if err != nil {
		return err
	}
	if n >= int64(t.AccountLimit) {
		return fmt.Errorf("account limit reached (%d)", t.AccountLimit)
	}
	return nil
}
