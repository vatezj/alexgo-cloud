package repository

import (
	"context"

	"gorm.io/gorm"

	"alexGo-cloud/modules/starfuyu/model"
	"alexGo-cloud/pkg/repo"
)

// DivinationRepository 求签记录数据访问（brief 钉死签名，Task 6 服务层唯一数据入口）。
// 用户表：不显式加 tenant 过滤——依赖 gormplugin + claims 写回的 ctx（scorecard 惯例）。
type DivinationRepository interface {
	// Create 落一条求签记录（签文已在签文缓存同号稳定），返回入库行。
	Create(ctx context.Context, in *model.Divination) (*model.Divination, error)
	// ListByMember 我的求签历史分页：created_at 倒序，total 为 member 全量条数。
	ListByMember(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error)
}

type divinationRepo struct{ db *gorm.DB }

func NewDivinationRepository(db *gorm.DB) DivinationRepository { return &divinationRepo{db: db} }

func (r *divinationRepo) Create(ctx context.Context, in *model.Divination) (*model.Divination, error) {
	// tenant_id 不硬编码：gormplugin 按 ctx 回填（tid=0 保持 0）。
	if err := r.db.WithContext(ctx).Create(in).Error; err != nil {
		return nil, err
	}
	return in, nil
}

func (r *divinationRepo) ListByMember(ctx context.Context, memberUserID uint64, page, size int) ([]*model.Divination, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.Divination{}).
		Where("member_user_id = ?", memberUserID).
		Order("created_at desc, id desc")
	return repo.Paginate[*model.Divination](ctx, q, page, size)
}
