package repository

import (
	"context"

	"alexGo-cloud/modules/order/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type CodegenDemoItemRepository interface {
	Create(ctx context.Context, entity *model.CodegenDemoItem) error
	List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoItem, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoItem, error)
	Update(ctx context.Context, entity *model.CodegenDemoItem) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type codegendemoitemRepo struct {
	db *gorm.DB
}

func NewCodegenDemoItemRepository(db *gorm.DB) CodegenDemoItemRepository {
	return &codegendemoitemRepo{db: db}
}

func (r *codegendemoitemRepo) Create(ctx context.Context, entity *model.CodegenDemoItem) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *codegendemoitemRepo) List(ctx context.Context, tenantID uint64) ([]*model.CodegenDemoItem, error) {
	var list []*model.CodegenDemoItem
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *codegendemoitemRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.CodegenDemoItem, error) {
	var entity model.CodegenDemoItem
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *codegendemoitemRepo) Update(ctx context.Context, entity *model.CodegenDemoItem) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.CodegenDemoItem{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *codegendemoitemRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.CodegenDemoItem{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.CodegenDemoItem{}).Error
}
