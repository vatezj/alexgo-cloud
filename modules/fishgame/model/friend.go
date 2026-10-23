package model

import "time"

// Friend 好友关系（spec §4.6；双向成对写，uk: player+friend）。
type Friend struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID       uint64    `gorm:"column:tenant_id" json:"-"`               // 租户ID
	PlayerID       uint64    `gorm:"column:player_id" json:"player_id"`
	FriendPlayerID uint64    `gorm:"column:friend_player_id" json:"friend_player_id"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Friend) TableName() string { return "fishgame_friend" }
