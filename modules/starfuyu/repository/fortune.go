package repository

import (
	"context"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type FortuneRepository interface {
	Create(ctx context.Context, entity *model.Fortune) error
	List(ctx context.Context, tenantID uint64) ([]*model.Fortune, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Fortune, error)
	Update(ctx context.Context, entity *model.Fortune) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type fortuneRepo struct {
	db *gorm.DB
}

func NewFortuneRepository(db *gorm.DB) FortuneRepository {
	return &fortuneRepo{db: db}
}

func (r *fortuneRepo) Create(ctx context.Context, entity *model.Fortune) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *fortuneRepo) List(ctx context.Context, tenantID uint64) ([]*model.Fortune, error) {
	var list []*model.Fortune
	q := r.db.WithContext(ctx).Model(&model.Fortune{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *fortuneRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Fortune, error) {
	var entity model.Fortune
	q := r.db.WithContext(ctx).Model(&model.Fortune{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *fortuneRepo) Update(ctx context.Context, entity *model.Fortune) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.Fortune{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *fortuneRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.Fortune{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.Fortune{}).Error
}
