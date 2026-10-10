package model

import (
	"time"
)

type CodegenDemoItem struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"` // 主键
	Name      string    `gorm:"column:name" json:"name"`                      // 名称
	Price     string    `gorm:"column:price" json:"price"`                    // 价格
	Status    int       `gorm:"column:status" json:"status"`                  // 状态
	Remark    string    `gorm:"column:remark" json:"remark"`                  // 备注
	Password  string    `gorm:"column:password" json:"-"`                     // 密码
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`          // 创建时间
	TenantID  uint64    `gorm:"column:tenant_id" json:"tenant_id"`            // 租户
}

func (CodegenDemoItem) TableName() string { return "codegen_demo_items" }
