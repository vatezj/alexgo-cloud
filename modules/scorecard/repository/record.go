package repository

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type RecordRepository interface {
	Create(ctx context.Context, entity *model.Record) error
	List(ctx context.Context, tenantID uint64) ([]*model.Record, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Record, error)
	Update(ctx context.Context, entity *model.Record) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type recordRepo struct {
	db *gorm.DB
}

func NewRecordRepository(db *gorm.DB) RecordRepository {
	return &recordRepo{db: db}
}

func (r *recordRepo) Create(ctx context.Context, entity *model.Record) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *recordRepo) List(ctx context.Context, tenantID uint64) ([]*model.Record, error) {
	var list []*model.Record
	q := r.db.WithContext(ctx).Model(&model.Record{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *recordRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.Record, error) {
	var entity model.Record
	q := r.db.WithContext(ctx).Model(&model.Record{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *recordRepo) Update(ctx context.Context, entity *model.Record) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.Record{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *recordRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.Record{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.Record{}).Error
}
