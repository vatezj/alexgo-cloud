package model

import "time"

// MemberUser 会员用户表 member_user（app 端 C 端账号，按 tenant_id 租户隔离）。
type MemberUser struct {
	ID         uint64     `gorm:"primaryKey" json:"id"`
	Nickname   string     `json:"nickname"`
	Avatar     string     `json:"avatar"`
	Status     int        `json:"status"` // 1启用 0停用
	Mobile     string     `json:"mobile"`
	Password   string     `json:"-"` // bcrypt
	RegisterIP string     `json:"register_ip"`
	LoginIP    string     `json:"login_ip"`
	LoginDate  *time.Time `json:"login_date"`
	Deleted    bool       `gorm:"column:deleted" json:"-"`
	TenantID   uint64     `json:"tenant_id"`
	// created_at/updated_at：迁移表用 yudao 风格 create_time/update_time 列，
	// 显式 column 标签对齐（同 pkg/token accessToken 的处理），JSON 名保持 created_at。
	CreatedAt time.Time `gorm:"column:create_time" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:update_time" json:"updated_at"`
}

// TableName 指定会员用户表名。
func (MemberUser) TableName() string { return "member_user" }
