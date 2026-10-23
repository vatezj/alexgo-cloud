package model

import "time"

// Player 游戏账号（openid 即账号，spec §4.8）。
type Player struct {
	ID          uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID    uint64    `gorm:"column:tenant_id" json:"-"`           // 租户ID（游戏端恒 0）
	OpenID      string    `gorm:"column:openid" json:"openid"`         // 微信 openid（uk含tenant）
	Nick        string    `gorm:"column:nick" json:"nick"`             // 昵称（授权资料占位：微信用户+尾4）
	Avatar      string    `gorm:"column:avatar" json:"avatar"`         // 头像 URL
	LastLoginAt time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Player) TableName() string { return "fishgame_player" }
