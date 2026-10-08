package model

import "time"

type Dept struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	ParentID  uint64    `json:"parent_id"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Dept) TableName() string { return "depts" }

type Post struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Post) TableName() string { return "posts" }

type DictType struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Status    int       `json:"status"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DictType) TableName() string { return "dict_types" }

type DictData struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TypeID    uint64    `json:"type_id"`
	Label     string    `json:"label"`
	Value     string    `json:"value"`
	Sort      int       `json:"sort"`
	Status    int       `json:"status"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DictData) TableName() string { return "dict_datas" }

type SystemConfig struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "system_configs" }

type SystemNotice struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    int       `json:"status"`
	TenantID  uint64    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemNotice) TableName() string { return "system_notices" }

