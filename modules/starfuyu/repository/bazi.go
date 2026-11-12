package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
)

// BaziRepository 我的八字数据访问（brief 钉死签名，Task 5 服务层唯一数据入口）。
// 用户表：不显式加 tenant 过滤——依赖 gormplugin + claims 写回的 ctx（scorecard 惯例）。
type BaziRepository interface {
	// GetByMember 按 member 查八字；软查 miss → (nil, nil)（service 映射 30504）。
	GetByMember(ctx context.Context, memberUserID uint64) (*model.Bazi, error)
	// Upsert (tenant_id, member_user_id) 幂等覆盖保存，返回整行。
	Upsert(ctx context.Context, memberUserID uint64, in *model.Bazi) (*model.Bazi, error)
}

type baziRepo struct{ db *gorm.DB }

func NewBaziRepository(db *gorm.DB) BaziRepository { return &baziRepo{db: db} }

func (r *baziRepo) GetByMember(ctx context.Context, memberUserID uint64) (*model.Bazi, error) {
	var out model.Bazi
	err := r.db.WithContext(ctx).Where("member_user_id = ?", memberUserID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *baziRepo) Upsert(ctx context.Context, memberUserID uint64, in *model.Bazi) (*model.Bazi, error) {
	in.MemberUserID = memberUserID // 入参 member 为准
	in.ID = 0                      // 冲突目标是 (tenant_id,member_user_id)，清主键防 PK 抢先命中
	// tenant_id 不硬编码：gormplugin 按 ctx 回填（tid=0 保持 0）。
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "member_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"birth_date", "birth_time", "birth_place", "bazi_info", "updated_at",
		}),
	}).Create(in).Error
	if err != nil {
		return nil, err
	}
	// OnConflict 后回填的 ID 在部分驱动不可靠，按 member 重读整行。
	return r.GetByMember(ctx, memberUserID)
}
