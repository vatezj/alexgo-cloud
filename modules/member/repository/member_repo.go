package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
)

// MemberRepository 会员用户仓储。
type MemberRepository interface {
	GetByMobile(ctx context.Context, tenantID uint64, mobile string) (*model.MemberUser, error)
	Create(ctx context.Context, u *model.MemberUser) error
	Update(ctx context.Context, u *model.MemberUser) error
	List(ctx context.Context, tenantID uint64, page, size int) ([]*model.MemberUser, int64, error)
	CountByTenant(ctx context.Context, tenantID uint64) (int64, error)
	// UpdateStatus 按 tenant+id 局部更新启用/停用状态（Disable 用；无按 ID 直取方法）。
	UpdateStatus(ctx context.Context, tenantID, id uint64, status int) error
}

type memberRepo struct{ db *gorm.DB }

// NewMemberRepository 构造 GORM 实现（fx 注入 *gorm.DB）。
func NewMemberRepository(db *gorm.DB) MemberRepository { return &memberRepo{db: db} }

func (r *memberRepo) GetByMobile(ctx context.Context, tid uint64, mobile string) (*model.MemberUser, error) {
	var u model.MemberUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND mobile = ? AND deleted = 0", tid, mobile).
		First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *memberRepo) Create(ctx context.Context, u *model.MemberUser) error {
	return r.db.WithContext(ctx).Create(u).Error
}

func (r *memberRepo) Update(ctx context.Context, u *model.MemberUser) error {
	return r.db.WithContext(ctx).Save(u).Error
}

func (r *memberRepo) List(ctx context.Context, tid uint64, page, size int) ([]*model.MemberUser, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	var total int64
	q := r.db.WithContext(ctx).Model(&model.MemberUser{}).Where("tenant_id = ? AND deleted = 0", tid)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []*model.MemberUser
	err := q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&items).Error
	return items, total, err
}

func (r *memberRepo) CountByTenant(ctx context.Context, tid uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.MemberUser{}).
		Where("tenant_id = ? AND deleted = 0", tid).Count(&n).Error
	return n, err
}

func (r *memberRepo) UpdateStatus(ctx context.Context, tid, id uint64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.MemberUser{}).
		Where("tenant_id = ? AND id = ? AND deleted = 0", tid, id).
		Update("status", status).Error
}
