package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/starfuyu/model"
)

// ProfileRepository 星座偏好数据访问（brief 钉死签名，Task 5 服务层唯一数据入口）。
// 用户表：不显式加 tenant 过滤——依赖 gormplugin + claims 写回的 ctx（scorecard 惯例）。
type ProfileRepository interface {
	// GetByMember 按 member 查偏好；软查 miss → (nil, nil)。
	GetByMember(ctx context.Context, memberUserID uint64) (*model.Profile, error)
	// Upsert (tenant_id, member_user_id) 幂等：存在则覆盖 constellation，返回整行。
	Upsert(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error)
}

type profileRepo struct{ db *gorm.DB }

func NewProfileRepository(db *gorm.DB) ProfileRepository { return &profileRepo{db: db} }

func (r *profileRepo) GetByMember(ctx context.Context, memberUserID uint64) (*model.Profile, error) {
	var out model.Profile
	err := r.db.WithContext(ctx).Where("member_user_id = ?", memberUserID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *profileRepo) Upsert(ctx context.Context, memberUserID uint64, constellation string) (*model.Profile, error) {
	p := &model.Profile{MemberUserID: memberUserID, Constellation: constellation}
	// tenant_id 不硬编码：gormplugin 按 ctx 回填（tid=0 保持 0）。
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "member_user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"constellation", "updated_at"}),
	}).Create(p).Error
	if err != nil {
		return nil, err
	}
	// OnConflict 后回填的 ID 在部分驱动不可靠，按 member 重读整行。
	return r.GetByMember(ctx, memberUserID)
}
