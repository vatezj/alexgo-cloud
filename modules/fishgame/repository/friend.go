package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"alexGo-cloud/modules/fishgame/model"
)

// FriendRepository 好友关系：双向成对写（uk: player+friend）。
type FriendRepository interface {
	AddPair(ctx context.Context, a, b uint64) error // 双向两行，幂等
	ListFriendIDs(ctx context.Context, playerID uint64) ([]uint64, error)
	Exists(ctx context.Context, playerID, friendID uint64) (bool, error)
}

type friendRepo struct{ db *gorm.DB }

func NewFriendRepository(db *gorm.DB) FriendRepository { return &friendRepo{db: db} }

func (r *friendRepo) AddPair(ctx context.Context, a, b uint64) error {
	rows := []*model.Friend{
		{TenantID: 0, PlayerID: a, FriendPlayerID: b},
		{TenantID: 0, PlayerID: b, FriendPlayerID: a},
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(rows).Error
}

func (r *friendRepo) ListFriendIDs(ctx context.Context, playerID uint64) ([]uint64, error) {
	var ids []uint64
	err := r.db.WithContext(ctx).Model(&model.Friend{}).
		Where("player_id = ?", playerID).Order("id asc").
		Pluck("friend_player_id", &ids).Error
	return ids, err
}

func (r *friendRepo) Exists(ctx context.Context, playerID, friendID uint64) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Friend{}).
		Where("player_id = ? AND friend_player_id = ?", playerID, friendID).Count(&n).Error
	return n > 0, err
}
