package model

import "time"

// Sign 签到对账（spec §4.1；(player,date) 唯一）。
type Sign struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	PlayerID  uint64    `gorm:"column:player_id;uniqueIndex:uk_sign_player_date,priority:1" json:"player_id"`
	Date      string    `gorm:"column:date;uniqueIndex:uk_sign_player_date,priority:2" json:"date"` // YYYY-MM-DD（uk: player+date）
	Streak    int       `gorm:"column:streak" json:"streak"`
	Bait      int       `gorm:"column:bait" json:"bait"` // 本次发放鱼饵
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Sign) TableName() string { return "fishgame_sign" }
