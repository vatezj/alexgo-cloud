package model

import "time"

// Session X-Session 自持会话（spec §4.8；与 oauth2 token 完全无关）。
type Session struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"`                          // 租户ID
	Token     string    `gorm:"column:token;uniqueIndex:uk_session_token" json:"-"` // 随机 32B base64url（唯一）
	PlayerID  uint64    `gorm:"column:player_id" json:"player_id"`
	ExpireAt  time.Time `gorm:"column:expire_at" json:"expire_at"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Session) TableName() string { return "fishgame_session" }
