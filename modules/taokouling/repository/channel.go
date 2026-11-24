package repository

import (
	"context"

	"alexGo-cloud/modules/taokouling/model"
	"alexGo-cloud/pkg/tenant"
	"gorm.io/gorm"
)

// ChannelRepository 渠道仓储（平台级数据，repo 自钉 tid=0，R1b）。
type ChannelRepository struct {
	db *gorm.DB
}

// NewChannelRepository 构造渠道仓储。
func NewChannelRepository(db *gorm.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

// List 分页列出渠道（id DESC，total 独立计数）。
func (r *ChannelRepository) List(ctx context.Context, page, size int) ([]model.Channel, int64, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&model.Channel{}).
		Where("tenant_id = ?", 0).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.Channel
	err := r.db.WithContext(ctx).Model(&model.Channel{}).
		Where("tenant_id = ?", 0).
		Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Create 新增渠道（强制 ch.TenantID=0）。
func (r *ChannelRepository) Create(ctx context.Context, ch *model.Channel) error {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	ch.TenantID = 0
	return r.db.WithContext(ctx).Create(ch).Error
}

// Update 按 ch.ID 全量更新（强制 TenantID 不变 0，行钉在 tid=0）。
func (r *ChannelRepository) Update(ctx context.Context, ch *model.Channel) error {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	ch.TenantID = 0
	// 禁用 Save：Select("*") 全量覆盖（含零值），双条件把行锁在 tid=0。
	return r.db.WithContext(ctx).Model(&model.Channel{}).
		Where("id = ? AND tenant_id = ?", ch.ID, 0).
		Select("*").Updates(ch).Error
}

// Delete 按 id 删除（行钉在 tid=0）。
func (r *ChannelRepository) Delete(ctx context.Context, id uint64) error {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, 0).
		Delete(&model.Channel{}).Error
}

// GetByID 按 id 取行；miss → (nil, nil)。
func (r *ChannelRepository) GetByID(ctx context.Context, id uint64) (*model.Channel, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	var list []model.Channel
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, 0).
		Limit(1).Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// GetEnabledByPartner 按 partner 取启用渠道（status=1，id asc limit 1）；
// partner=="" → (nil, nil)。
func (r *ChannelRepository) GetEnabledByPartner(ctx context.Context, partner string) (*model.Channel, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	if partner == "" {
		return nil, nil
	}
	var list []model.Channel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND partner = ? AND status = ?", 0, partner, 1).
		Order("id ASC").Limit(1).Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// GetEnabledByHost 按 host 取启用渠道（status=1，id asc limit 1）；
// host=="" → (nil, nil)。
func (r *ChannelRepository) GetEnabledByHost(ctx context.Context, host string) (*model.Channel, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	if host == "" {
		return nil, nil
	}
	var list []model.Channel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND host = ? AND status = ?", 0, host, 1).
		Order("id ASC").Limit(1).Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// GetDefault 默认渠道（partner=” AND host=” AND status=1）；miss → (nil, nil)。
func (r *ChannelRepository) GetDefault(ctx context.Context) (*model.Channel, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	var list []model.Channel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND partner = ? AND host = ? AND status = ?", 0, "", "", 1).
		Limit(1).Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, nil
	}
	return &list[0], nil
}

// ExistsPair (tenant_id,partner,host) 查重；excludeID>0 时排除自身。
func (r *ChannelRepository) ExistsPair(ctx context.Context, partner, host string, excludeID uint64) (bool, error) {
	ctx = tenant.WithTenantID(ctx, 0) // 平台级数据，repo 自钉 0
	q := r.db.WithContext(ctx).Model(&model.Channel{}).
		Where("tenant_id = ? AND partner = ? AND host = ?", 0, partner, host)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}
