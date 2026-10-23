package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/fishgame/model"
)

// PlayerRepository 游戏账号数据访问。游戏端 ctx 无租户注入，写路径显式 tenant_id=0。
type PlayerRepository interface {
	// UpsertByOpenID (tenant_id, openid) 幂等：存在则更新 nick/avatar/last_login_at，返回整行。
	UpsertByOpenID(ctx context.Context, openid, nick, avatar string) (*model.Player, error)
	GetByID(ctx context.Context, id uint64) (*model.Player, error) // 软查：不存在 → (nil, nil)
	GetByOpenID(ctx context.Context, openid string) (*model.Player, error)
}

type playerRepo struct{ db *gorm.DB }

func NewPlayerRepository(db *gorm.DB) PlayerRepository { return &playerRepo{db: db} }

func (r *playerRepo) UpsertByOpenID(ctx context.Context, openid, nick, avatar string) (*model.Player, error) {
	now := time.Now()
	p := &model.Player{TenantID: 0, OpenID: openid, Nick: nick, Avatar: avatar, LastLoginAt: now}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "openid"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"nick":          nick,
			"avatar":        avatar,
			"last_login_at": now,
			"updated_at":    now,
		}),
	}).Create(p).Error
	if err != nil {
		return nil, err
	}
	// OnConflict 后 GORM 回填的 ID 在部分驱动不可靠，按 openid 重读整行。
	return r.GetByOpenID(ctx, openid)
}

func (r *playerRepo) GetByID(ctx context.Context, id uint64) (*model.Player, error) {
	var out model.Player
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}

func (r *playerRepo) GetByOpenID(ctx context.Context, openid string) (*model.Player, error) {
	var out model.Player
	err := r.db.WithContext(ctx).Where("openid = ?", openid).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &out, err
}
