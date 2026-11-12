package model

import "time"

// SignCache 签文按签号持久缓存（决定 15：首访生成后按签号缓存，draw 与公开
// GET 共用；签文是全局内容，tenant_id 恒 0）。解读不入本表（实时 DeepSeek + 限流）。
type SignCache struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID    uint64    `gorm:"column:tenant_id;uniqueIndex:uk_sign_number,priority:1" json:"-"`             // 租户ID（恒 0）
	SignNumber  int       `gorm:"column:sign_number;uniqueIndex:uk_sign_number,priority:2" json:"sign_number"` // 签号 1-100
	SignContent string    `gorm:"column:sign_content;type:text" json:"sign_content"`                           // 签文内容（同号稳定）
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SignCache) TableName() string { return "starfuyu_sign_cache" }
