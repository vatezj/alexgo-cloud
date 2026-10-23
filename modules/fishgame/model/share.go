package model

import "time"

// Share 分享图（spec §4.6；key 随机，图落对象存储，DB 只存元数据）。
type Share struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"`                      // 租户ID
	Key       string    `gorm:"column:key;uniqueIndex:uk_share_key" json:"key"` // 随机 key（唯一，路径 /share/<key>）
	PlayerID  uint64    `gorm:"column:player_id" json:"player_id"`
	Size      int64     `gorm:"column:size" json:"size"` // 字节数
	Mime      string    `gorm:"column:mime" json:"mime"` // image/jpeg|image/png
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Share) TableName() string { return "fishgame_share" }
