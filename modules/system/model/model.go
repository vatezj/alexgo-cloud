package model

import "time"

type User struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	Username     string     `json:"username"`
	Nickname     string     `json:"nickname"`
	PasswordHash string     `json:"-"`
	Remark       string     `json:"remark"`
	DeptID       uint64     `json:"dept_id"`
	PostIDs      string     `json:"post_ids"`
	Email        string     `json:"email"`
	Mobile       string     `json:"mobile"`
	Sex          int        `json:"sex"`
	Avatar       string     `json:"avatar"`
	Status       int        `json:"status"`
	LoginIP      string     `json:"login_ip"`
	LoginDate    *time.Time `json:"login_date"`
	Deleted      bool       `gorm:"column:deleted" json:"-"`
	TenantID     uint64     `json:"tenant_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName：users 已在迁移 20261008000001 重命名为 system_users。
func (User) TableName() string { return "system_users" }
