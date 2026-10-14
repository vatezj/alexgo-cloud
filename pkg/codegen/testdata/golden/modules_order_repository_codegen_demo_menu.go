package repository

import (
	"context"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type CodegenDemoMenuRepository interface {
	Create(ctx context.Context, entity *model.CodegenDemoMenu) error
	List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoMenu, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoMenu, error)
	Update(ctx context.Context, entity *model.CodegenDemoMenu) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
	// HasChildren 树表专用（spec §10.2）：id 是否有子节点（根 parent_id=0 的子也计入）。
	HasChildren(ctx context.Context, tenantID uint64, id uint64) (bool, error)
}

type codegendemomenuRepo struct {
	db *gorm.DB
}

func NewCodegenDemoMenuRepository(db *gorm.DB) CodegenDemoMenuRepository {
	return &codegendemomenuRepo{db: db}
}

func (r *codegendemomenuRepo) Create(ctx context.Context, entity *model.CodegenDemoMenu) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *codegendemomenuRepo) List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoMenu, error) {
	var list []*model.CodegenDemoMenu
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoMenu{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *codegendemomenuRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoMenu, error) {
	var entity model.CodegenDemoMenu
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoMenu{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *codegendemomenuRepo) Update(ctx context.Context, entity *model.CodegenDemoMenu) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.CodegenDemoMenu{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *codegendemomenuRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoMenu{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.CodegenDemoMenu{}).Error
}

func (r *codegendemomenuRepo) HasChildren(ctx context.Context, tenantID uint64, id uint64) (bool, error) {
	var count int64
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoMenu{}).Where("parent_id = ?", id)
	q = q.Where("tenant_id = ?", tenantID)
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
