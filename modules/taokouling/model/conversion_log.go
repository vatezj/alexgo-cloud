package model

import (
	"time"
)

// ConversionLog 淘口令转换记录（平台级数据，tenant_id 恒 0）。
type ConversionLog struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id" json:"-"`                 // 租户ID（恒0）
	Endpoint     string    `gorm:"column:endpoint" json:"endpoint"`           // parse|convert
	Partner      string    `gorm:"column:partner" json:"partner"`             // 解析到的partner短码
	InputText    string    `gorm:"column:input_text" json:"input_text"`       // 用户输入（截断500）
	ExtractedTkl string    `gorm:"column:extracted_tkl" json:"extracted_tkl"` // 提取的口令体
	ItemID       string    `gorm:"column:item_id" json:"item_id"`             // 商品ID（可空串）
	Status       int8      `gorm:"column:status" json:"status"`               // 1成功 2降级 3无口令 4硬错误
	Error        string    `gorm:"column:error" json:"error"`                 // 失败原因（可空串）
	DurationMs   int       `gorm:"column:duration_ms" json:"duration_ms"`     // 耗时毫秒
	IP           string    `gorm:"column:ip" json:"ip"`                       // 客户端IP
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`       // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`       // 更新时间
}

// TableName 钉表名。
func (ConversionLog) TableName() string { return "taokouling_conversion_log" }
