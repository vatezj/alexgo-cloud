package model

import "time"

// Fish 图鉴条目（spec §5；fish_id 是后端唯一认可标识，取自客户端 fish_db.gd ENTRIES）。
type Fish struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id;uniqueIndex:uk_fish_fish_id,priority:1" json:"-"` // 租户ID
	FishID    string    `gorm:"column:fish_id;uniqueIndex:uk_fish_fish_id,priority:2" json:"fish_id"`
	Name      string    `gorm:"column:name" json:"name"`
	Rarity    string    `gorm:"column:rarity" json:"rarity"` // normal|misc|rare|epic
	Period    string    `gorm:"column:period" json:"period"` // any|day|dusk|night|组合
	Depth     string    `gorm:"column:depth" json:"depth"`   // any|shallow|mid|deep
	Flavor    string    `gorm:"column:flavor" json:"flavor"`
	Sort      int       `gorm:"column:sort" json:"sort"`   // 插入序 1-48
	Status    int       `gorm:"column:status" json:"status"` // 1=启用 0=停用
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Fish) TableName() string { return "fishgame_fish" }
