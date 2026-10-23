package model

import "time"

// FriendBlack 拉黑：UserID 拉黑 BlockedUserID（单向行；判定时查双向）。
type FriendBlack struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID      uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	UserID        uint64    `gorm:"column:user_id;uniqueIndex:uk_friend_black" json:"user_id"`
	BlockedUserID uint64    `gorm:"column:blocked_user_id;uniqueIndex:uk_friend_black" json:"blocked_user_id"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定拉黑关系表名（单数，与迁移 DDL 一致）。
func (FriendBlack) TableName() string { return "scorecard_friend_black" }
