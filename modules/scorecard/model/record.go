package model

import (
	"time"
)

type Record struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"`           // 租户ID
	Name      string    `gorm:"column:name" json:"name"`             // 名称
	Status    int       `gorm:"column:status" json:"status"`         // 状态 1=启用 0=停用
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"` // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"` // 更新时间
}

func (Record) TableName() string { return "records" }
