package model

import "time"

// Dex 玩家图鉴进度（spec §4.3；点亮数 = 行数 where count≥1）。
type Dex struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	PlayerID  uint64    `gorm:"column:player_id" json:"player_id"`
	FishID    string    `gorm:"column:fish_id" json:"fish_id"` // uk: player+fish
	Count     int       `gorm:"column:count" json:"count"`     // 钓获次数
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Dex) TableName() string { return "fishgame_dex" }
