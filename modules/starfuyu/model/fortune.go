package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// LuckyColor 幸运颜色（JSON 列；goLucky models/fortune.go 的 Valuer/Scanner 模式迁入）。
type LuckyColor struct {
	Chinese string `json:"chinese"` // 中文色名
	English string `json:"english"` // 英文色名
	Hex     string `json:"hex"`     // #RRGGBB
}

// Scan 实现 sql.Scanner（MySQL json 列取回 []byte，sqlite 部分路径为 string）。
func (lc *LuckyColor) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, lc)
	case string:
		return json.Unmarshal([]byte(v), lc)
	default:
		return fmt.Errorf("starfuyu: LuckyColor 无法从 %T 扫描", value)
	}
}

// Value 实现 driver.Valuer。
func (lc LuckyColor) Value() (driver.Value, error) {
	return json.Marshal(lc)
}

// Fortune 每日星座运势内容（spec §5；内容表：读写钉 tenant_id=0，决定 6）。
type Fortune struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID      uint64     `gorm:"column:tenant_id;uniqueIndex:uk_fortune_date_const,priority:1" json:"-"`       // 租户ID（恒 0）
	Date          string     `gorm:"column:date;uniqueIndex:uk_fortune_date_const,priority:2;size:10" json:"date"` // YYYY-MM-DD
	Constellation string     `gorm:"column:constellation;uniqueIndex:uk_fortune_date_const,priority:3;size:20" json:"constellation"`
	DailyOverview string     `gorm:"column:daily_overview;type:text" json:"daily_overview"` // 每日概述
	OverallScore  int        `gorm:"column:overall_score" json:"overall_score"`             // 综合运势 0-100
	LoveScore     int        `gorm:"column:love_score" json:"love_score"`                   // 爱情运势
	CareerScore   int        `gorm:"column:career_score" json:"career_score"`               // 事业运势
	WealthScore   int        `gorm:"column:wealth_score" json:"wealth_score"`               // 财运
	HealthScore   int        `gorm:"column:health_score" json:"health_score"`               // 健康运势
	LuckyColor    LuckyColor `gorm:"column:lucky_color;type:json" json:"lucky_color"`       // 幸运颜色 JSON
	LuckyNumber   int        `gorm:"column:lucky_number" json:"lucky_number"`               // 幸运数字
	Summary       string     `gorm:"column:summary;type:text" json:"summary"`               // 今日总评
	LoveTip       string     `gorm:"column:love_tip;type:text" json:"love_tip"`             // 爱情小贴士
	CareerTip     string     `gorm:"column:career_tip;type:text" json:"career_tip"`         // 事业小贴士
	WealthTip     string     `gorm:"column:wealth_tip;type:text" json:"wealth_tip"`         // 财运小贴士
	HealthTip     string     `gorm:"column:health_tip;type:text" json:"health_tip"`         // 健康小贴士
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (Fortune) TableName() string { return "starfuyu_fortune" }
