package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/fishgame/model"
	"alexGo-cloud/pkg/repo"
)

// PlayerRepository 游戏账号数据访问。游戏端 ctx 无租户注入，写路径显式 tenant_id=0。
type PlayerRepository interface {
	// UpsertByOpenID (tenant_id, openid) 幂等：存在则更新 nick/avatar/last_login_at，返回整行。
	UpsertByOpenID(ctx context.Context, openid, nick, avatar string) (*model.Player, error)
	GetByID(ctx context.Context, id uint64) (*model.Player, error) // 软查：不存在 → (nil, nil)
	GetByOpenID(ctx context.Context, openid string) (*model.Player, error)
	// ListAll 全体玩家（好友榜空回退口径，spec §4.6）；租户插件按 ctx tid 过滤。
	ListAll(ctx context.Context) ([]*model.Player, error)
	// ListAdmin 玩家分页：keyword openid/nick 模糊（admin /players，spec §4.2）。
	ListAdmin(ctx context.Context, keyword string, page, size int) ([]*model.Player, int64, error)
	CountAll(ctx context.Context) (int64, error)                           // 看板：玩家总数
	CountCreatedSince(ctx context.Context, since time.Time) (int64, error) // 看板：since 起新增
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

func (r *playerRepo) ListAll(ctx context.Context) ([]*model.Player, error) {
	var list []*model.Player
	err := r.db.WithContext(ctx).Order("id asc").Find(&list).Error
	return list, err
}

// ---- admin 聚合（Task 9）----

// ListAdmin 玩家分页：keyword 非空按 openid/nick 模糊，倒序（spec §4.2 /players）。
func (r *playerRepo) ListAdmin(ctx context.Context, keyword string, page, size int) ([]*model.Player, int64, error) {
	q := r.db.Model(&model.Player{}).Order("id desc")
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("openid LIKE ? OR nick LIKE ?", like, like)
	}
	return repo.Paginate[*model.Player](ctx, q, page, size)
}

// CountAll 玩家总数（看板）。
func (r *playerRepo) CountAll(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Player{}).Count(&n).Error
	return n, err
}

// CountCreatedSince since 起注册数（看板「今日新增」，按 player.created_at）。
func (r *playerRepo) CountCreatedSince(ctx context.Context, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Player{}).
		Where("created_at >= ?", since).Count(&n).Error
	return n, err
}
