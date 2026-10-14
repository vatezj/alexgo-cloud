package model

import (
	"time"
)

type CodegenDemoMenu struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"` // 主键
	ParentID  uint64    `gorm:"column:parent_id" json:"parent_id"`            // 父级ID
	Name      string    `gorm:"column:name" json:"name"`                      // 名称
	Remark    string    `gorm:"column:remark" json:"remark"`                  // 备注
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`          // 创建时间
	TenantID  uint64    `gorm:"column:tenant_id" json:"-"`                    // 租户
}

func (CodegenDemoMenu) TableName() string { return "codegen_demo_menus" }
