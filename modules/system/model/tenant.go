package model

import "time"

// Tenant 租户（全局表，不带 tenant_id——gormplugin ExemptTables 白名单）。
// 迁移 20261008000004_tenants 用 yudao 风格 create_time/update_time 列，
// 显式 column 标签对齐（同 member/token 模型处理），JSON 名保持 created_at。
type Tenant struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	Name         string     `json:"name"`
	PackageID    uint64     `gorm:"column:package_id" json:"package_id"`
	Status       int        `json:"status"` // 1启用 0停用
	ExpireTime   *time.Time `gorm:"column:expire_time" json:"expire_time"`
	AccountLimit int        `gorm:"column:account_limit" json:"account_limit"`
	Domain       string     `json:"domain"`
	Deleted      bool       `gorm:"column:deleted" json:"-"`
	Creator      string     `json:"creator,omitempty"`
	Updater      string     `json:"updater,omitempty"`
	CreatedAt    time.Time  `gorm:"column:create_time" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:update_time" json:"updated_at"`
}

// TableName 指定租户表名。
func (Tenant) TableName() string { return "tenants" }
