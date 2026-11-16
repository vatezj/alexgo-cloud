package model

import "time"

// DailyHoroscope 当日全局运势头图数据：重点星象 + 开篇总述（图片版软文顶部两块）。
// 每日一条（uk tenant_id+date），与 Fortune（每星座一行）正交；DeepSeek 抓取时随 12 星座一并生成。
type DailyHoroscope struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID  uint64    `gorm:"column:tenant_id;uniqueIndex:uk_daily_tenant_date,priority:1" json:"-"` // 租户ID（恒 0）
	Date      string    `gorm:"column:date;uniqueIndex:uk_daily_tenant_date,priority:2;size:10" json:"date"`
	Astro     string    `gorm:"column:astro;size:64" json:"astro"` // 重点星象（如「月水星，月土六合，木火合」）
	Intro     string    `gorm:"column:intro" json:"intro"`         // 开篇总述（≤60 字）
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

// TableName 指定表名（gorm 默认复数化会拼错）。
func (DailyHoroscope) TableName() string { return "starfuyu_daily" }
