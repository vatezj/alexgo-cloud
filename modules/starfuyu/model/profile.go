package model

import "time"

// Profile 用户星座偏好（spec §5；决策 7：不给 member_user 加列，偏好落本表）。
type Profile struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TenantID      uint64    `gorm:"column:tenant_id;uniqueIndex:uk_profile_member,priority:1" json:"-"`                   // 租户ID
	MemberUserID  uint64    `gorm:"column:member_user_id;uniqueIndex:uk_profile_member,priority:2" json:"member_user_id"` // 归属 member 用户（uk 含 tenant）
	Constellation string    `gorm:"column:constellation;size:20" json:"constellation"`                                    // 12 星座枚举值（校验在 Task 5）
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Profile) TableName() string { return "starfuyu_profile" }
