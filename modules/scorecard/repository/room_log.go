package repository

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type RoomLogRepository interface {
	Create(ctx context.Context, entity *model.RoomLog) error
	List(ctx context.Context, tenantID uint64) ([]*model.RoomLog, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.RoomLog, error)
	Update(ctx context.Context, entity *model.RoomLog) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type roomlogRepo struct {
	db *gorm.DB
}

func NewRoomLogRepository(db *gorm.DB) RoomLogRepository {
	return &roomlogRepo{db: db}
}

func (r *roomlogRepo) Create(ctx context.Context, entity *model.RoomLog) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *roomlogRepo) List(ctx context.Context, tenantID uint64) ([]*model.RoomLog, error) {
	var list []*model.RoomLog
	q := r.db.WithContext(ctx).Model(&model.RoomLog{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *roomlogRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.RoomLog, error) {
	var entity model.RoomLog
	q := r.db.WithContext(ctx).Model(&model.RoomLog{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *roomlogRepo) Update(ctx context.Context, entity *model.RoomLog) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.RoomLog{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *roomlogRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.RoomLog{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.RoomLog{}).Error
}
