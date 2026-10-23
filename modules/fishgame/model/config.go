package model

import "time"

// Config 运营覆盖（spec §4.7；type=luck 或 fish_king，优先于确定性自算）。
type Config struct {
	ID        uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64     `gorm:"column:tenant_id" json:"-"`           // 租户ID
	Type      string     `gorm:"column:type" json:"type"`             // luck|fish_king
	Period    string     `gorm:"column:period" json:"period"`         // luck：day|dusk|night|any
	DepthBias float64    `gorm:"column:depth_bias" json:"depth_bias"` // luck：≤0.3
	FishID    string     `gorm:"column:fish_id" json:"fish_id"`       // fish_king：图鉴 id
	Week      string     `gorm:"column:week" json:"week"`             // fish_king：ISO 周 YYYY-Www
	StartAt   *time.Time `gorm:"column:start_at" json:"start_at"`     // 可空=立即生效
	EndAt     *time.Time `gorm:"column:end_at" json:"end_at"`         // 可空=永久
	Status    int        `gorm:"column:status" json:"status"`         // 1=启用 0=停用
	CreatedAt time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Config) TableName() string { return "fishgame_config" }
