package model

import "time"

// Divination 求签记录（spec §5；签文取自 starfuyu_sign_cache 同号稳定，解读实时生成后落库）。
type Divination struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID       uint64    `gorm:"column:tenant_id;index:idx_div_member_created,priority:1" json:"-"`                   // 租户ID
	MemberUserID   uint64    `gorm:"column:member_user_id;index:idx_div_member_created,priority:2" json:"member_user_id"` // 归属 member 用户
	BaziID         *uint64   `gorm:"column:bazi_id" json:"bazi_id"`                                                       // 关联八字（可空：无八字走降级路径）
	Question       string    `gorm:"column:question;type:text" json:"question"`                                           // 求签问题（可空）
	SignNumber     int       `gorm:"column:sign_number" json:"sign_number"`                                               // 签号 1-100
	SignContent    string    `gorm:"column:sign_content;type:text" json:"sign_content"`                                   // 签文（缓存命中值）
	Interpretation string    `gorm:"column:interpretation;type:text" json:"interpretation"`                               // 签文解读（实时生成）
	Category       string    `gorm:"column:category;size:20" json:"category"`                                             // 签文类别 事业/爱情/健康/财运/综合
	LuckyDirection string    `gorm:"column:lucky_direction;size:50" json:"lucky_direction"`                               // 幸运方位
	LuckyColor     string    `gorm:"column:lucky_color;size:50" json:"lucky_color"`                                       // 幸运颜色（文本，非运势表 JSON）
	Advice         string    `gorm:"column:advice;type:text" json:"advice"`                                               // 具体建议
	CreatedAt      time.Time `gorm:"column:created_at;index:idx_div_member_created,priority:3" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Divination) TableName() string { return "starfuyu_divination" }
