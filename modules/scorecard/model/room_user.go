package model

import "time"

// RoomUser 房间成员。cum_lose_score 本会话累计输分（退房重进经 session_no 重置）。
type RoomUser struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	RoomID       uint64    `gorm:"column:room_id;uniqueIndex:uk_room_user" json:"room_id"`
	UserID       uint64    `gorm:"column:user_id;uniqueIndex:uk_room_user" json:"user_id"`
	CurScore     int       `gorm:"column:cur_score" json:"cur_score"`
	CumLoseScore int       `gorm:"column:cum_lose_score" json:"cum_lose_score"`
	IsCreator    int       `gorm:"column:is_creator" json:"is_creator"`
	SessionNo    int       `gorm:"column:session_no" json:"session_no"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定房间成员表名（单数，与迁移 DDL 一致）。
func (RoomUser) TableName() string { return "room_user" }
