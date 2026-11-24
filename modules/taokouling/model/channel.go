package model

import (
	"time"
)

// Channel 淘口令转换渠道（平台级数据，tenant_id 恒 0；复合唯一键 (tenant_id,partner,host)）。
type Channel struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id;uniqueIndex:uk_tenant_partner_host" json:"-"`     // 租户ID（恒0）
	Name      string    `gorm:"column:name" json:"name"`                                          // 渠道名称
	Partner   string    `gorm:"column:partner;uniqueIndex:uk_tenant_partner_host" json:"partner"` // partner短码（空=非partner渠道）
	Host      string    `gorm:"column:host;uniqueIndex:uk_tenant_partner_host" json:"host"`       // 匹配Host（空=不匹配）
	PID       string    `gorm:"column:pid" json:"pid"`                                            // 淘宝联盟推广位
	APIKey    string    `gorm:"column:apikey" json:"apikey"`                                      // 好单库apikey
	TBName    string    `gorm:"column:tb_name" json:"tb_name"`                                    // 淘宝账号昵称
	Status    int8      `gorm:"column:status" json:"status"`                                      // 状态 1=启用 0=停用
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`                              // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`                              // 更新时间
}

// TableName 钉表名。
func (Channel) TableName() string { return "taokouling_channel" }
