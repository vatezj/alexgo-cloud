package model

import "time"

// Room 房间。Status: 0 进行中 1 已结算 2 已关闭。pending_* 为「从下把生效」暂存字段。
type Room struct {
	ID                     uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID               uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	RoomCode               string    `gorm:"column:room_code;size:8;uniqueIndex:uk_room_code" json:"room_code"`
	CreatorID              uint64    `gorm:"column:creator_id" json:"creator_id"`
	Status                 int       `gorm:"column:status" json:"status"`
	ExpireTime             time.Time `gorm:"column:expire_time" json:"expire_time"`
	TeaTargetAmount        int       `gorm:"column:tea_target_amount" json:"tea_target_amount"`
	TeaPercent             float64   `gorm:"column:tea_percent" json:"tea_percent"`
	TeaPoolTotal           int       `gorm:"column:tea_pool_total" json:"tea_pool_total"`
	TeaAutoEnabled         int       `gorm:"column:tea_auto_enabled" json:"tea_auto_enabled"`
	MaxLoseScore           int       `gorm:"column:max_lose_score" json:"max_lose_score"`
	RoundNo                int       `gorm:"column:round_no" json:"round_no"`
	PendingTeaTargetAmount int       `gorm:"column:pending_tea_target_amount" json:"pending_tea_target_amount"`
	PendingTeaPercent      float64   `gorm:"column:pending_tea_percent" json:"pending_tea_percent"`
	PendingMaxLoseScore    int       `gorm:"column:pending_max_lose_score" json:"pending_max_lose_score"`
	PendingApplyRoundNo    int       `gorm:"column:pending_apply_round_no" json:"pending_apply_round_no"`
	Version                int       `gorm:"column:version" json:"version"`
	CreatedAt              time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定房间表名（单数，与迁移 DDL 一致）。
func (Room) TableName() string { return "scorecard_room" }
