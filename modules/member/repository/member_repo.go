package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/member/model"
	"alexGo-cloud/pkg/repo"
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

// List 分页列出（id desc 保序）。page/size 由 controller Defaults() 保证，
// repo.Paginate 内部仅做防御兜底；签名不变（service/controller 不动）。
func (r *memberRepo) List(ctx context.Context, tid uint64, page, size int) ([]*model.MemberUser, int64, error) {
	q := r.db.Model(&model.MemberUser{}).
		Where("tenant_id = ? AND deleted = 0", tid).
		Order("id desc")
	return repo.Paginate[*model.MemberUser](ctx, q, page, size)
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
