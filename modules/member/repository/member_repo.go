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
	// ListAll 租户内全量（导出用；不截断、不走 Paginate——后者 page/size≤0 兜底 1/10）。
	ListAll(ctx context.Context, tenantID uint64) ([]*model.MemberUser, error)
	// UpdateStatus 按 tenant+id 局部更新启用/停用状态（Disable 用；无按 ID 直取方法）。
	UpdateStatus(ctx context.Context, tenantID, id uint64, status int) error
	// GetBriefs 按 id 批量取会员摘要（跨模块窄只读，BriefProvider 消费）：
	// 窄列 + deleted=0 口径与本包其余查询一致；租户隔离依赖 gormplugin 按 ctx 注入。
	GetBriefs(ctx context.Context, ids []uint64) ([]*model.MemberUser, error)
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

// ListAll 租户内全量导出行：与 List 同 Where 口径（tenant_id + deleted=0），
// id desc 保序；无 Offset/Limit——导出要的是全量，不是分页。
func (r *memberRepo) ListAll(ctx context.Context, tid uint64) ([]*model.MemberUser, error) {
	var list []*model.MemberUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deleted = 0", tid).
		Order("id desc").
		Find(&list).Error
	return list, err
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

// GetBriefs 按 id 批量取会员摘要行：只 SELECT 摘要列（不带 mobile/password 等敏感列），
// deleted=0 与本包其余查询同口径（model.Deleted 是普通 bool 列、非 gorm.DeletedAt，
// 不会自动过滤）；id asc 保证批量取数输出确定序。租户过滤不显式写——gormplugin
// 按 ctx 自动注入（tid=0 不注入，平台侧行为同插件设计）。
func (r *memberRepo) GetBriefs(ctx context.Context, ids []uint64) ([]*model.MemberUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []*model.MemberUser
	err := r.db.WithContext(ctx).
		Select([]string{"id", "nickname", "avatar", "status"}).
		Where("id IN ? AND deleted = 0", ids).
		Order("id").
		Find(&list).Error
	return list, err
}
