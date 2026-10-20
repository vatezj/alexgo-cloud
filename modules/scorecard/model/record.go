package model

import "time"

// Record 战绩：每成员一行。SettleType: 1 正常结算 2 房主退房自动结算 3 退房 4 零数据空战绩。
// 房间终局配置冗余快照，统计免 JOIN。
type Record struct {
	ID              uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID        uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	RoomID          uint64    `gorm:"column:room_id;index:idx_record_room" json:"room_id"`
	UserID          uint64    `gorm:"column:user_id;index:idx_record_user_created,priority:1" json:"user_id"`
	Score           int       `gorm:"column:score" json:"score"`
	CumLoseScore    int       `gorm:"column:cum_lose_score" json:"cum_lose_score"`
	TeaPoolTotal    int       `gorm:"column:tea_pool_total" json:"tea_pool_total"`
	TeaPercent      float64   `gorm:"column:tea_percent" json:"tea_percent"`
	TeaTargetAmount int       `gorm:"column:tea_target_amount" json:"tea_target_amount"`
	MaxLoseScore    int       `gorm:"column:max_lose_score" json:"max_lose_score"`
	SettleType      int       `gorm:"column:settle_type" json:"settle_type"`
	CreatedAt       time.Time `gorm:"column:created_at;index:idx_record_user_created,priority:2" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定战绩表名（单数，与迁移 DDL 一致）。
func (Record) TableName() string { return "record" }
