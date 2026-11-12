package model

import "time"

// RequestLimit 每日请求限流行（spec §5；uk: tenant+member+date+type）。
type RequestLimit struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;uniqueIndex:uk_limit_user_date_type,priority:1" json:"-"`                   // 租户ID
	MemberUserID uint64    `gorm:"column:member_user_id;uniqueIndex:uk_limit_user_date_type,priority:2" json:"member_user_id"` // 归属 member 用户
	Date         string    `gorm:"column:date;uniqueIndex:uk_limit_user_date_type,priority:3;size:10" json:"date"`             // YYYY-MM-DD（上海时区，决定 18）
	Type         string    `gorm:"column:type;uniqueIndex:uk_limit_user_date_type,priority:4;size:20" json:"type"`             // 限流类型见下方常量
	Count        int       `gorm:"column:count" json:"count"`                                                                  // 当日已用次数
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RequestLimit) TableName() string { return "starfuyu_request_limit" }

// 限流类型（spec §5：type ∈ bazi_calculate / divination_draw）。
const (
	RequestTypeBaziCalculate  = "bazi_calculate"  // 八字计算
	RequestTypeDivinationDraw = "divination_draw" // 求签
)
