package model

import "time"

// TrackEvent 埋点事实流（spec §4.4；追加型，dedup_key 唯一防重发膨胀）。
type TrackEvent struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"` // 租户ID
	PlayerID  uint64    `gorm:"column:player_id" json:"player_id"`
	Name      string    `gorm:"column:name" json:"name"` // 事件名（catch/sign_in/...）
	T         int64     `gorm:"column:t" json:"t"`       // 客户端本地 epoch 秒
	Props     string    `gorm:"column:props" json:"props"` // JSON 文本（≤4KB 截断）
	DedupKey  string    `gorm:"column:dedup_key" json:"-"` // sha1(openid|name|t|canonical_json)
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (TrackEvent) TableName() string { return "fishgame_track_event" }
