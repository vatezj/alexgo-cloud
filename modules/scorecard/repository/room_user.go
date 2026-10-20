package repository

import (
	"context"

	"alexGo-cloud/modules/scorecard/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

type RoomUserRepository interface {
	Create(ctx context.Context, entity *model.RoomUser) error
	List(ctx context.Context, tenantID uint64) ([]*model.RoomUser, error)
	GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.RoomUser, error)
	Update(ctx context.Context, entity *model.RoomUser) error
	Delete(ctx context.Context, tenantID uint64, id uint64) error
}

type roomuserRepo struct {
	db *gorm.DB
}

func NewRoomUserRepository(db *gorm.DB) RoomUserRepository {
	return &roomuserRepo{db: db}
}

func (r *roomuserRepo) Create(ctx context.Context, entity *model.RoomUser) error {
	// 打戳（spec §9.2）：json:"-" 使客户端灌不进来，零值才回填、显式值保留。
	if entity.TenantID == 0 {
		entity.TenantID = tenant.TenantIDFromContext(ctx)
	}
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *roomuserRepo) List(ctx context.Context, tenantID uint64) ([]*model.RoomUser, error) {
	var list []*model.RoomUser
	q := r.db.WithContext(ctx).Model(&model.RoomUser{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Find(&list).Error
	return list, err
}

func (r *roomuserRepo) GetByID(ctx context.Context, tenantID uint64, id uint64) (*model.RoomUser, error) {
	var entity model.RoomUser
	q := r.db.WithContext(ctx).Model(&model.RoomUser{})
	q = q.Where("tenant_id = ?", tenantID)
	err := q.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (r *roomuserRepo) Update(ctx context.Context, entity *model.RoomUser) error {
	// 禁用 Save（spec §9.2）：Save 丢弃链式 WHERE，跨租户照改不误。
	// Select("*") 保证零值也写入（全量覆盖语义），双条件把行锁在本租户内。
	return r.db.WithContext(ctx).Model(&model.RoomUser{}).
		Where("id = ? AND tenant_id = ?", entity.ID, tenant.TenantIDFromContext(ctx)).
		Select("*").Updates(entity).Error
}

func (r *roomuserRepo) Delete(ctx context.Context, tenantID uint64, id uint64) error {
	q := r.db.WithContext(ctx).Model(&model.RoomUser{})
	q = q.Where("tenant_id = ?", tenantID)
	return q.Where("id = ?", id).Delete(&model.RoomUser{}).Error
}
