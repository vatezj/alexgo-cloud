package model

import "time"

// Friend 好友关系（单向关系行，双向各存一行由 service 保证）；RankEnabled 0/1 参与排名。
type Friend struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	UserID       uint64    `gorm:"column:user_id;uniqueIndex:uk_friend" json:"user_id"`
	FriendUserID uint64    `gorm:"column:friend_user_id;uniqueIndex:uk_friend" json:"friend_user_id"`
	RankEnabled  int       `gorm:"column:rank_enabled" json:"rank_enabled"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定好友关系表名（单数，与迁移 DDL 一致）。
func (Friend) TableName() string { return "scorecard_friend" }
