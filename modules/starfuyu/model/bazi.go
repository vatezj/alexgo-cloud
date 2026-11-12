package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// WuxingInfo 五行信息（JSON 内嵌结构，goLucky models/bazi.go 同构）。
type WuxingInfo struct {
	YearWuxing  string         `json:"year_wuxing"`  // 年柱五行
	MonthWuxing string         `json:"month_wuxing"` // 月柱五行
	DayWuxing   string         `json:"day_wuxing"`   // 日柱五行
	HourWuxing  string         `json:"hour_wuxing"`  // 时柱五行
	TotalWuxing map[string]int `json:"total_wuxing"` // 五行统计 {"金":2,"木":1,...}
}

// BaziInfo 八字信息（四柱 + 五行 + 节气/农历/生肖/星座；goLucky models/bazi.go 同构）。
type BaziInfo struct {
	YearGan  string `json:"year_gan"`  // 年干
	YearZhi  string `json:"year_zhi"`  // 年支
	MonthGan string `json:"month_gan"` // 月干
	MonthZhi string `json:"month_zhi"` // 月支
	DayGan   string `json:"day_gan"`   // 日干
	DayZhi   string `json:"day_zhi"`   // 日支
	HourGan  string `json:"hour_gan"`  // 时干
	HourZhi  string `json:"hour_zhi"`  // 时支

	Wuxing WuxingInfo `json:"wuxing"` // 五行信息

	SolarTerm     string `json:"solar_term"`    // 节气
	LunarDate     string `json:"lunar_date"`    // 农历日期
	Zodiac        string `json:"zodiac"`        // 生肖
	Constellation string `json:"constellation"` // 星座
}

// Scan 实现 sql.Scanner（MySQL json 列取回 []byte，sqlite 部分路径为 string）。
func (b *BaziInfo) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return json.Unmarshal(v, b)
	case string:
		return json.Unmarshal([]byte(v), b)
	default:
		return fmt.Errorf("starfuyu: BaziInfo 无法从 %T 扫描", value)
	}
}

// Value 实现 driver.Valuer。
func (b BaziInfo) Value() (driver.Value, error) {
	return json.Marshal(b)
}

// Bazi 我的八字（spec §5；uk: tenant+member，覆盖保存）。
type Bazi struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID     uint64    `gorm:"column:tenant_id;uniqueIndex:uk_bazi_member,priority:1" json:"-"`                   // 租户ID
	MemberUserID uint64    `gorm:"column:member_user_id;uniqueIndex:uk_bazi_member,priority:2" json:"member_user_id"` // 归属 member 用户（uk 含 tenant）
	BirthDate    string    `gorm:"column:birth_date;size:10" json:"birth_date"`                                       // 出生日期 YYYY-MM-DD
	BirthTime    string    `gorm:"column:birth_time;size:8" json:"birth_time"`                                        // 出生时间 HH:MM:SS
	BirthPlace   string    `gorm:"column:birth_place;size:100" json:"birth_place"`                                    // 出生地点（可选）
	BaziInfo     BaziInfo  `gorm:"column:bazi_info;type:json" json:"bazi_info"`                                       // 八字信息 JSON
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Bazi) TableName() string { return "starfuyu_bazi" }
