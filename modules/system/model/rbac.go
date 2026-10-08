package model

import "time"

type Role struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	Code             string    `json:"code"`
	Name             string    `json:"name"`
	Sort             int       `json:"sort"`
	DataScope        int       `json:"data_scope"`
	DataScopeDeptIDs string    `json:"data_scope_dept_ids"`
	Type             int       `json:"type"`
	Remark           string    `json:"remark"`
	Status           int       `json:"status"`
	Deleted          bool      `gorm:"column:deleted" json:"-"`
	Creator          string    `json:"creator,omitempty"`
	Updater          string    `json:"updater,omitempty"`
	TenantID         uint64    `json:"tenant_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Menu struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	ParentID   uint64    `json:"parent_id"`
	Type       string    `json:"type"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Component  string    `json:"component"`
	Icon       string    `json:"icon"`
	Permission string    `json:"permission"`
	Sort       int       `json:"sort"`
	Status     int       `json:"status"`
	Deleted    bool      `gorm:"column:deleted" json:"-"`
	Updater    string    `json:"updater,omitempty"`
	Creator    string    `json:"creator,omitempty"`
	TenantID   uint64    `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type UserRole struct {
	ID       uint64 `gorm:"primaryKey" json:"id"`
	UserID   uint64 `json:"user_id"`
	RoleID   uint64 `json:"role_id"`
	TenantID uint64 `json:"tenant_id"`
}

type RoleMenu struct {
	ID       uint64 `gorm:"primaryKey" json:"id"`
	RoleID   uint64 `json:"role_id"`
	MenuID   uint64 `json:"menu_id"`
	TenantID uint64 `json:"tenant_id"`
}
