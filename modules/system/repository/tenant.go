package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/system/model"
)

// TenantRepository 租户持久化（tenants 全局表）。
type TenantRepository interface {
	List(ctx context.Context) ([]*model.Tenant, error)
	GetByID(ctx context.Context, id uint64) (*model.Tenant, error)
	Create(ctx context.Context, t *model.Tenant) error
	Update(ctx context.Context, t *model.Tenant) error
	Delete(ctx context.Context, id uint64) error
	CountAccounts(ctx context.Context, tenantID uint64) (int64, error)
}

type tenantRepo struct{ db *gorm.DB }

// NewTenantRepository 构造租户仓储。
func NewTenantRepository(db *gorm.DB) TenantRepository { return &tenantRepo{db: db} }

func (r *tenantRepo) List(ctx context.Context) ([]*model.Tenant, error) {
	var items []*model.Tenant
	err := r.db.WithContext(ctx).Where("deleted = 0").Order("id desc").Find(&items).Error
	return items, err
}

func (r *tenantRepo) GetByID(ctx context.Context, id uint64) (*model.Tenant, error) {
	var item model.Tenant
	err := r.db.WithContext(ctx).Where("id = ? AND deleted = 0", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *tenantRepo) Create(ctx context.Context, t *model.Tenant) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *tenantRepo) Update(ctx context.Context, t *model.Tenant) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// Delete 软删（tenants 在 gormplugin 白名单内，不参与租户过滤）。
func (r *tenantRepo) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&model.Tenant{}).
		Where("id = ?", id).Update("deleted", 1).Error
}

// CountAccounts 统计租户下 system_users + member_user 账号数（raw SQL：
// member 表可能不存在于纯 system 场景——用存在性容错，缺表按 0 计）。
func (r *tenantRepo) CountAccounts(ctx context.Context, tenantID uint64) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Table("system_users").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&n).Error; err != nil {
		return 0, err
	}
	var m int64
	err := r.db.WithContext(ctx).Table("member_user").
		Where("tenant_id = ? AND deleted = 0", tenantID).Count(&m).Error
	if err == nil {
		n += m
	}
	return n, nil
}
